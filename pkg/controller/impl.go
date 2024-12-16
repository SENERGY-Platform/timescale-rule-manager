/*
 * Copyright 2023 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package controller

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/SENERGY-Platform/models/go/models"
	perm "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/config"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/database"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/model"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/security"
	"github.com/hashicorp/go-uuid"
	"golang.org/x/exp/slices"
)

type impl struct {
	db                          database.DB
	permv2                      perm.Client
	oidClient                   *security.Client
	kafkaTopicTableUpdates      string
	kafkaTopicPermissionUpdates string
	deviceIdPrefix              string
	serviceIdPrefix             string
	fatal                       func(error)
	mux                         sync.Mutex
	debug                       bool
	slowMuxLock                 time.Duration
}

func New(c config.Config, db database.DB, permv2 perm.Client, fatal func(error), ctx context.Context, wg *sync.WaitGroup) (Controller, error) {
	oidClient, err := security.NewClient(c.KeycloakUrl, c.KeycloakClientId, c.KeycloakClientSecret)
	if err != nil {
		return nil, err
	}
	slowMuxLock := 0 * time.Nanosecond
	if len(c.SlowMuxLock) > 0 {
		slowMuxLock, err = time.ParseDuration(c.SlowMuxLock)
		if err != nil {
			return nil, err
		}
	}
	controller := &impl{db: db, permv2: permv2, oidClient: oidClient, deviceIdPrefix: c.DeviceIdPrefix, serviceIdPrefix: c.ServiceIdPrefix, mux: sync.Mutex{}, fatal: fatal, debug: c.Debug, slowMuxLock: slowMuxLock}
	err = controller.setupKafka(c, ctx, wg)
	if err != nil {
		return nil, err
	}
	return controller, err
}

func (this *impl) CreateRule(rule *model.Rule) (res *model.TypedRule, code int, err error) {
	myRule := rule.Copy()
	if len(myRule.Id) != 0 {
		return nil, http.StatusBadRequest, errors.New("may not specify Id yourself")
	}
	myRule.Id, err = uuid.GenerateUUID()
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	myRule.CompletedRun = false
	tx, cancel, err := this.db.GetTx()
	defer cancel()
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	err = this.db.InsertRule(&myRule, tx)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	err = tx.Commit()
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	runRule := myRule.Copy()
	go this.runRule(&runRule)
	typed, err := myRule.Type()
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return typed, http.StatusOK, nil
}
func (this *impl) UpdateRule(rule *model.Rule) (code int, err error) {
	tx, cancel, err := this.db.GetTx()
	defer cancel()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	rule.CompletedRun = false
	err = this.db.UpdateRule(rule, tx)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return http.StatusNotFound, err
		}
		return http.StatusInternalServerError, err
	}
	err = tx.Commit()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	go this.runRule(rule)
	return http.StatusOK, nil
}
func (this *impl) DeleteRule(id string) (code int, err error) {
	err = this.lock()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	this.logDebug("locked db for DeleteRule " + id)
	defer func() {
		err := this.unlock()
		if err != nil {
			log.Println("FATAL: Could not unlock postgresql. Exiting to avoid deadlock!")
			this.fatal(err)
		}
		this.logDebug("unlocked db for DeleteRule " + id)
	}()
	tx, cancel, err := this.db.GetTx()
	defer cancel()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	tables, err := this.db.FindMatchingTables([]string{id}, tx)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	for _, table := range tables {
		allRanOk, code, err := this.applyRulesForTable(table, true, []string{id}, tx)
		if err != nil {
			return code, err
		}
		if !allRanOk {
			return http.StatusBadRequest, errors.New("rule has delete template that finished with errors. " +
				"Will not delete rule to avoid inconsistencies")
		}
	}
	err = this.db.DeleteRule(id, tx)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return http.StatusNotFound, err
		}
		return http.StatusInternalServerError, err
	}
	err = tx.Commit()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}
func (this *impl) GetRule(id string) (typedRule *model.TypedRule, code int, err error) {
	tx, cancel, err := this.db.GetTx()
	defer cancel()
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	rule, err := this.db.GetRule(id, tx)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, http.StatusNotFound, err
		}
		return nil, http.StatusInternalServerError, err
	}
	typed, err := rule.Type()
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return typed, http.StatusOK, nil
}
func (this *impl) ListRules(limit, offset int) (typedRules []model.TypedRule, code int, err error) {
	rules, err := this.db.ListRules(limit, offset)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	typedRules = []model.TypedRule{}
	for _, rule := range rules {
		rule := rule
		typed, err := rule.Type()
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		typedRules = append(typedRules, *typed)
	}
	return typedRules, http.StatusOK, nil
}

var exportTableMatch = regexp.MustCompile("userid:(.{22})_export:(.{22}).*")
var deviceTableMatch = regexp.MustCompile("device:(.{22})_service:(.{22}).*")

func (this *impl) ApplyAllRulesForTable(table string, useDeleteTemplateInstead bool) (code int, err error) {
	err = this.lock()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	this.logDebug("locked db for ApplyAllRulesForTable " + table)
	defer func() {
		err := this.unlock()
		if err != nil {
			log.Println("FATAL: Could not unlock postgresql. Exiting to avoid deadlock!")
			this.fatal(err)
		}
		this.logDebug("unlocked db for ApplyAllRulesForTable " + table)
	}()
	tx, cancel, err := this.db.GetTx()
	defer cancel()
	_, code, err = this.applyRulesForTable(table, useDeleteTemplateInstead, nil, tx)
	if err != nil {
		return code, err
	}
	err = tx.Commit()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return code, err
}

func (this *impl) applyRulesForTable(table string, useDeleteTemplateInstead bool, limitToRuleIds []string, tx *sql.Tx) (allRanOk bool, code int, err error) {
	if limitToRuleIds != nil {
		this.logDebug("applying rules to table " + table + " limited to rule ids " + strings.Join(limitToRuleIds, ", "))
	} else {
		this.logDebug("applying rules to table " + table + " unlimited to any rule ids")
	}

	allRanOk = true
	tableInfo := model.TableInfo{Table: table, Roles: []string{}}
	matches := exportTableMatch.FindAllStringSubmatch(table, -1)
	if matches != nil && len(matches[0]) == 3 { // is export table
		this.logDebug(table + " is an export table")
		tableInfo.ShortUserId = matches[0][1]
		longUserId, err := models.LongId(tableInfo.ShortUserId)
		if err != nil {
			return false, http.StatusInternalServerError, err
		}
		tableInfo.UserIds = []string{longUserId}
		tableInfo.ShortExportId = matches[0][2]
		tableInfo.ExportId, err = models.LongId(tableInfo.ShortExportId)
		if err != nil {
			return false, http.StatusInternalServerError, err
		}
	} else {
		matches = deviceTableMatch.FindAllStringSubmatch(table, -1)
		if matches != nil && len(matches[0]) == 3 { // is device-service table
			this.logDebug(table + " is a device table")
			tableInfo.ShortDeviceId = matches[0][1]
			longDeviceId, err := models.LongId(tableInfo.ShortDeviceId)
			if err != nil {
				return false, http.StatusInternalServerError, err
			}
			tableInfo.DeviceId = this.deviceIdPrefix + longDeviceId
			tableInfo.ShortServiceId = matches[0][2]
			longServiceId, err := models.LongId(tableInfo.ShortServiceId)
			if err != nil {
				return false, http.StatusInternalServerError, err
			}
			tableInfo.ServiceId = this.serviceIdPrefix + longServiceId
			// get Device Owners
			token, err := this.oidClient.GetToken()
			if err != nil {
				return false, http.StatusInternalServerError, err
			}
			resource, err, _ := this.permv2.GetResource(token.JwtToken(), "devices", tableInfo.DeviceId)
			if err != nil {
				err = errors.New(err.Error() + tableInfo.DeviceId)
				return false, http.StatusInternalServerError, err
			}
			tableInfo.Roles = []string{}
			for group, groupRights := range resource.RolePermissions { // groups are roles...
				if groupRights.Execute {
					tableInfo.Roles = append(tableInfo.Roles, group)
				}
			}
			for userId, userRights := range resource.UserPermissions {
				if userRights.Execute {
					tableInfo.UserIds = append(tableInfo.UserIds, userId)
				}
			}

		} else {
			return false, http.StatusBadRequest, errors.New("unknown table format")
		}
	}

	for _, userId := range tableInfo.UserIds {
		realmRoleMappings, err := this.oidClient.GetRealmRoleMappings(userId)
		if err != nil {
			err = errors.New(err.Error() + ", userId: " + userId)
			return false, http.StatusInternalServerError, err
		}
		for _, realmRoleMapping := range realmRoleMappings {
			if !slices.Contains(tableInfo.Roles, realmRoleMapping.Name) {
				tableInfo.Roles = append(tableInfo.Roles, realmRoleMapping.Name)
			}
		}
	}

	this.logDebug(table + " belongs to users " + strings.Join(tableInfo.UserIds, ", ") + " and roles " + strings.Join(tableInfo.Roles, ", "))
	rules, err := this.db.FindMatchingRulesWithOwnerInfo(table, tableInfo.UserIds, tableInfo.Roles, limitToRuleIds, tx)
	if err != nil {
		return false, http.StatusInternalServerError, err
	}

	if len(rules) > 0 {
		tableInfo.Columns, err = this.db.GetColumns(table)
		if err != nil {
			return false, http.StatusInternalServerError, err
		}
	}

	for _, rule := range rules {
		this.logDebug("applying rule " + rule.Id + " to table " + table)
		t := rule.CommandTemplate
		if useDeleteTemplateInstead {
			t = rule.DeleteTemplate
		}
		savepoint := "rule"
		_, err = tx.Exec("SAVEPOINT " + savepoint + ";")
		if err != nil {
			return false, http.StatusInternalServerError, err
		}
		errorhandling := func(ruleErr error) error {
			allRanOk = false
			_, err = tx.Exec("ROLLBACK TO SAVEPOINT " + savepoint + ";")
			if err != nil {
				return err
			}
			if rule.Errors == nil {
				rule.Errors = []string{}
			}
			rule.Errors = append(rule.Errors, table+": "+ruleErr.Error())
			err = this.db.UpdateRule(&rule, tx)
			return err
		}
		tmpl, err := template.New("").Parse(t)
		if err != nil {
			err = errorhandling(err)
			if err != nil {
				return false, http.StatusInternalServerError, err
			}
			continue
		}
		query, err := execTempl(tmpl, tableInfo)
		if err != nil {
			err = errorhandling(err)
			if err != nil {
				return false, http.StatusInternalServerError, err
			}
			continue
		}
		_, err = this.db.Exec(query, tx)
		if err != nil {
			err = errorhandling(err)
			if err != nil {
				return false, http.StatusInternalServerError, err
			}
			continue
		}
	}

	return allRanOk, http.StatusOK, nil
}

func (this *impl) ApplyAllRules() error {
	err := this.lock()
	if err != nil {
		return err
	}
	this.logDebug("locked db for ApplyAllRules")
	defer func() {
		err := this.unlock()
		if err != nil {
			log.Println("FATAL: Could not unlock postgresql. Exiting to avoid deadlock!")
			this.fatal(err)
		}
		this.logDebug("unlocked db for ApplyAllRules")
	}()
	limit := 1000
	offset := 0
	tx, cancel, err := this.db.GetTx()
	if err != nil {
		return err
	}
	defer cancel()
	for {
		rules, _, err := this.ListRules(limit, offset)
		if err != nil {
			return err
		}

		ruleIds := make([]string, len(rules))
		for i, rule := range rules {
			ruleIds[i] = rule.Id
		}
		tables, err := this.db.FindMatchingTables(ruleIds, tx)
		if err != nil {
			return err
		}

		for _, table := range tables {
			allOk, _, err := this.applyRulesForTable(table, false, ruleIds, tx)
			if err != nil {
				return err
			}
			if !allOk {
				log.Println("WARN: Not all rules for table " + table + " could be applied without errors")
			}
		}

		offset += len(rules)
		if len(rules) < limit {
			break
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (this *impl) runRule(rule *model.Rule) {
	this.logDebug("running rule " + rule.Id)
	err := this.lock()
	if err != nil {
		log.Println("ERROR: ", err)
		return
	}
	this.logDebug("locked db for rule " + rule.Id)
	defer func() {
		err := this.unlock()
		if err != nil {
			log.Println("FATAL: Could not unlock postgresql. Exiting to avoid deadlock!")
			this.fatal(err)
		}
		this.logDebug("unlocked db for rule " + rule.Id)
	}()
	rule.Errors = []string{}
	tx, cancel, err := this.db.GetTx()
	defer cancel()
	if err != nil {
		log.Println("ERROR: ", err)
		return
	}
	rollbackAndSave := func(rule *model.Rule) {
		log.Println("rolling back rule "+rule.Id, rule.Errors)
		err = tx.Rollback()
		if err != nil {
			log.Println("ERROR", err)
		}
		err = this.saveRule(rule)
		if err != nil {
			log.Println("ERROR", err)
		}
	}
	tables, err := this.db.FindMatchingTables([]string{rule.Id}, tx)
	if err != nil {
		rule.Errors = append(rule.Errors, err.Error())
		rollbackAndSave(rule)
		return
	}
	this.logDebug("for rule " + rule.Id + " found tables " + strings.Join(tables, ", "))

	for _, table := range tables {
		_, _, err := this.applyRulesForTable(table, false, []string{rule.Id}, tx)
		if err != nil {
			rule.Errors = append(rule.Errors, err.Error())
			rollbackAndSave(rule)
			return
		}
		rule, err = this.db.GetRule(rule.Id, tx)
		if err != nil {
			rule.Errors = append(rule.Errors, err.Error())
			rollbackAndSave(rule)
			return
		}
		if rule.Errors != nil && len(rule.Errors) > 0 {
			rollbackAndSave(rule)
			return
		}
	}

	this.logDebug("rule " + rule.Id + " finished run")
	rule.CompletedRun = true
	err = this.db.UpdateRule(rule, tx)
	if err != nil {
		log.Println("ERROR", err)
		return
	}

	err = tx.Commit()
	if err != nil {
		log.Println("ERROR", err)
		return
	}
	this.logDebug("rule " + rule.Id + " finished run, committed changes")

	return
}

func (this *impl) saveRule(rule *model.Rule) error {
	tx, cancel, err := this.db.GetTx()
	defer cancel()
	if err != nil {
		return err
	}
	err = this.db.UpdateRule(rule, tx)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (this *impl) lock() error {
	time.Sleep(this.slowMuxLock)
	this.mux.Lock()
	this.logDebug("internal mux locked, attemtping db mux lock")
	err := this.db.Lock()
	if err != nil {
		this.logDebug("db mux locked with error \n" + string(debug.Stack()))
		this.mux.Unlock()
	}
	this.logDebug("db mux locked\n" + string(debug.Stack()))
	return err
}

func (this *impl) unlock() error {
	this.mux.Unlock()
	this.logDebug("mux unlocked")
	return this.db.Unlock()
}

func (this *impl) logDebug(s string) {
	if this.debug {
		log.Println("DEBUG: " + s)
	}
}

func execTempl(t *template.Template, value any) (string, error) {
	buf := &bytes.Buffer{}
	err := t.Execute(buf, value)
	return buf.String(), err
}
