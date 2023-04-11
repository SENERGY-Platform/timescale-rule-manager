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

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/SENERGY-Platform/models/go/models"
	_ "github.com/lib/pq"
	"github.com/senergy-platform/timescale-rule-manager/pkg/model"
	"log"
	"strings"
	"sync"
	"time"
)

type impl struct {
	ruleSchema string
	ruleTable  string
	sql        *sql.DB
	ctx        context.Context
	debug      bool
}

func New(postgresHost string, postgresPort int, postgresUser string, postgresPw string, postgresDb string,
	postgresRuleSchema string, postgresRuleTable string, debug bool, ctx context.Context, wg *sync.WaitGroup) (DB, error) {
	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", postgresHost,
		postgresPort, postgresUser, postgresPw, postgresDb)
	log.Println("Connecting to PSQL...", psqlconn)
	// open database
	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		return nil, err
	}

	wg.Add(1)
	go func() {
		<-ctx.Done()
		_ = db.Close()
		wg.Done()
	}()

	err = db.Ping()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	i := &impl{sql: db, ctx: ctx, ruleSchema: postgresRuleSchema, ruleTable: postgresRuleTable, debug: debug}
	return i, i.migrate()
}

func (this *impl) GetTx() (tx *sql.Tx, cancel context.CancelFunc, err error) {
	ctx, cancel := context.WithTimeout(this.ctx, time.Second*30)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	tx, err = this.sql.BeginTx(ctx, nil)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return tx, cancel, nil
}

func (this *impl) InsertRule(rule *model.Rule, tx *sql.Tx) (err error) {
	query := fmt.Sprintf("INSERT INTO \"%s\".\"%s\" (", this.ruleSchema, this.ruleTable)
	fields, values := getFieldsAndValues(rule)
	valueStr := "VALUES ("
	for i := range fields {
		if i > 0 {
			query += ", "
			valueStr += ", "
		}
		query += "\"" + fields[i] + "\""
		valueStr += values[i]
	}
	valueStr += ")"
	query += ") " + valueStr + ";"
	_, err = tx.Exec(query)
	if err != nil {
		return err
	}
	return nil
}

func (this *impl) UpdateRule(rule *model.Rule, tx *sql.Tx) (err error) {
	query := fmt.Sprintf("UPDATE \"%s\".\"%s\" SET ", this.ruleSchema, this.ruleTable)
	fields, values := getFieldsAndValues(rule)
	for i := range fields {
		if i > 0 {
			query += ", "
		}
		query += "\"" + fields[i] + "\" = " + values[i]
	}
	query += " WHERE \"Id\" = '" + rule.Id + "';"
	res, err := tx.Exec(query)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (this *impl) DeleteRule(id string, tx *sql.Tx) (err error) {
	res, err := tx.Exec(fmt.Sprintf("DELETE FROM  \"%s\".\"%s\" WHERE \"Id\" = '%s';", this.ruleSchema, this.ruleTable, id))
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (this *impl) GetRule(id string, tx *sql.Tx) (rule *model.Rule, err error) {
	r := tx.QueryRow(fmt.Sprintf("SELECT * FROM \"%s\".\"%s\" WHERE \"Id\" = '%s'", this.ruleSchema, this.ruleTable, id))
	rule = &model.Rule{}
	err = scan(r, rule)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return rule, nil
}

func (this *impl) ListRules(limit, offset int) (rules []model.Rule, err error) {
	rows, err := this.sql.Query(fmt.Sprintf("SELECT * FROM \"%s\".\"%s\" LIMIT %d OFFSET %d",
		this.ruleSchema, this.ruleTable, limit, offset))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		rule := model.Rule{}
		err = scan(rows, &rule)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (this *impl) FindMatchingTables(ruleIds []string, tx *sql.Tx) (tables []string, err error) {
	query := fmt.Sprintf("SELECT information_schema.tables.table_name "+
		"FROM information_schema.tables, \"%s\".\"%s\" WHERE information_schema.tables.table_schema = 'public' AND information_schema.tables.table_name ~ \"%s\".\"%s\".\"TableRegEx\" AND \"%s\".\"%s\".\"Id\"IN ('"+strings.Join(ruleIds, "', '")+"');",
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
	)
	return this.queryStrings(query, tx)
}

func (this *impl) FindMatchingRules(tables []string, tx *sql.Tx) (rules []model.Rule, err error) {
	query := fmt.Sprintf("SELECT \"%s\".\"%s\".* "+
		"FROM information_schema.tables, \"%s\".\"%s\" WHERE information_schema.tables.table_schema = 'public' AND information_schema.tables.table_name ~ \"%s\".\"%s\".\"TableRegEx\" AND information_schema.tables.table_name IN ('"+strings.Join(tables, "', '")+"');",
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
	)
	rows, err := tx.Query(query)
	if err != nil {
		return nil, err
	}
	rules = []model.Rule{}
	for rows.Next() {
		rule := model.Rule{}
		err = scan(rows, &rule)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (this *impl) FindDeviceTables(deviceId string) (tables []string, err error) {
	shortDeviceId, err := models.ShortenId(deviceId)
	if err != nil {
		return nil, err
	}
	query := "SELECT table_name FROM information_schema.tables WHERE table_name like 'device:" + shortDeviceId + "%';"
	return this.queryStrings(query, this.sql)
}

func (this *impl) GetColumns(table string) (columns []string, err error) {
	query := fmt.Sprintf("SELECT column_name FROM information_schema.columns where table_name = '%s' AND table_schema = 'public';", table)
	return this.queryStrings(query, this.sql)
}

func (this *impl) FindMatchingRulesWithOwnerInfo(table string, userIds []string, roles []string, limitToRuleIds []string, tx *sql.Tx) (rules []model.Rule, err error) {
	query := fmt.Sprintf("SELECT DISTINCT ON (\"%s\".\"%s\".\"Group\") \"%s\".\"%s\".* "+ // only one rule per Group
		"FROM information_schema.tables, \"%s\".\"%s\" WHERE information_schema.tables.table_schema = 'public' "+ // table is in schema public
		"AND information_schema.tables.table_name ~ \"%s\".\"%s\".\"TableRegEx\" "+ // table matches rule regex
		"AND information_schema.tables.table_name = '%s' "+ // table name matches
		"AND ("+ // roles or user matches
		"	\"%s\".\"%s\".\"Roles\" && ARRAY['"+strings.Join(roles, "', '")+"'] "+ // any roles overlap
		"	OR \"%s\".\"%s\".\"Users\" && ARRAY['"+strings.Join(userIds, "', '")+"']"+ // any userIds overlap
		")", // ensures DISTINCT ON selects rule with the highest Priority per Group
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
		table,
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
	)
	if limitToRuleIds != nil {
		query += fmt.Sprintf(" AND \"%s\".\"%s\".\"Id\" IN ('"+strings.Join(limitToRuleIds, "', '")+"')",
			this.ruleSchema, this.ruleTable,
		)
	}
	query += " ORDER BY \"Group\", \"Priority\" DESC;"
	rows, err := tx.Query(query)
	if err != nil {
		return nil, err
	}
	rules = []model.Rule{}
	for rows.Next() {
		rule := model.Rule{}
		err = scan(rows, &rule)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (this *impl) Exec(query string, tx *sql.Tx) (sql.Result, error) {
	if this.debug {
		log.Println(query)
	}
	return tx.Exec(query)
}
