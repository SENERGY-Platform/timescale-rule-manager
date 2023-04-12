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
	"database/sql"
	"github.com/senergy-platform/timescale-rule-manager/pkg/model"
	"reflect"
)

func (this *impl) migrate() (err error) {
	return this.withTx(func(tx *sql.Tx) error {
		query := "CREATE SCHEMA IF NOT EXISTS " + this.ruleSchema + ";"
		_, err = tx.Exec(query)
		if err != nil {
			return err
		}

		query = this.getMigrationQuery()
		_, err = tx.Exec(query)
		if err != nil {
			return err
		}
		return nil
	})
}

func (this *impl) getMigrationQuery() string {
	t := reflect.TypeOf(model.Rule{})
	query := "CREATE TABLE IF NOT EXISTS \"" + this.ruleSchema + "\".\"" + this.ruleTable + "\" (\n"
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		sqlType, ok := field.Tag.Lookup("sqltype")
		if !ok {
			continue
		}
		if i > 0 {
			query += ",\n"
		}
		query += "\"" + field.Name + "\" " + sqlType
		sqlExtra, ok := field.Tag.Lookup("sqlextra")
		if !ok {
			continue
		}
		query += " " + sqlExtra

	}
	query += "\n);"
	query += "\nALTER TABLE \"" + this.ruleSchema + "\".\"" + this.ruleTable + "\" ADD COLUMN IF NOT EXISTS \"CompletedRun\" boolean not null default false;"
	return query
}
