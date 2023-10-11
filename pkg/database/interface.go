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
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/model"
)

type DB interface {
	GetTx() (tx *sql.Tx, cancel context.CancelFunc, err error)
	InsertRule(rule *model.Rule, tx *sql.Tx) (err error)
	UpdateRule(rule *model.Rule, tx *sql.Tx) (err error)
	DeleteRule(id string, tx *sql.Tx) (err error)
	GetRule(id string, tx *sql.Tx) (rule *model.Rule, err error)
	ListRules(limit, offset int) (rules []model.Rule, err error)
	FindMatchingTables(ruleIds []string, tx *sql.Tx) (tables []string, err error)
	FindMatchingRules(tables []string, tx *sql.Tx) (rules []model.Rule, err error)
	FindMatchingRulesWithOwnerInfo(table string, userIds []string, roles []string, limitToRuleIds []string, tx *sql.Tx) (rules []model.Rule, err error)
	FindDeviceTables(deviceId string) (tables []string, err error)
	GetColumns(table string) (columns []string, err error)
	Exec(query string, tx *sql.Tx) (result sql.Result, err error)
}

var ErrNotFound = errors.New("not found")
