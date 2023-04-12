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
	"github.com/lib/pq"
	"github.com/senergy-platform/timescale-rule-manager/pkg/model"
	"reflect"
	"strings"
	"time"
)

func (this *impl) withTx(f func(tx *sql.Tx) error) (err error) {
	ctx, cancel := context.WithTimeout(this.ctx, time.Second*30)
	defer cancel()
	tx, err := this.sql.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		} else {
			err = tx.Commit()
		}
	}()
	err = f(tx)
	return err
}

type queryable interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

func (this *impl) queryStrings(query string, tx queryable) (result []string, err error) {
	rows, err := tx.Query(query)
	if err != nil {
		return nil, err
	}
	result = []string{}
	for rows.Next() {
		var s string
		err = rows.Scan(&s)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, nil
}

func getFieldsAndValues(rule *model.Rule) (fields []string, values []string) {
	fields = []string{}
	values = []string{}
	t := reflect.TypeOf(*rule)
	v := reflect.ValueOf(*rule)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		_, ok := field.Tag.Lookup("sqltype")
		if !ok {
			continue
		}
		fields = append(fields, field.Name)
		fv := v.FieldByName(field.Name)
		value := ""
		switch fv.Interface().(type) {
		case string:
			value = fmt.Sprintf("'%s'", strings.ReplaceAll(fv.String(), "'", "''"))
		case int, int64:
			value = fmt.Sprintf("'%d'", fv.Int())
		case float64, float32:
			value = fmt.Sprintf("'%f'", fv.Float())
		case []string:
			arr := fv.Interface().([]string)
			value = "'{"
			for j, s := range arr {
				if j > 0 {
					value += ", "
				}
				value += "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
			}
			value += "}'"
		case *bool:
			b := fv.Interface().(*bool)
			if b == nil {
				value = "NULL"
			} else if *b {
				value = "TRUE"
			} else {
				value = "FALSE"
			}
		case bool:
			b := fv.Interface().(bool)
			if b {
				value = "TRUE"
			} else {
				value = "FALSE"
			}
		default:
			value = "NULL"
		}
		values = append(values, value)
	}
	return fields, values
}

type scannable interface {
	Scan(dest ...any) error
}

func scan(r scannable, rule *model.Rule, other ...any) error {
	if other == nil {
		other = []interface{}{}
	}
	other = append(other, &rule.Id, &rule.Description, &rule.Priority, &rule.Group, &rule.TableRegEx,
		(*pq.StringArray)(&rule.Users), (*pq.StringArray)(&rule.Roles), &rule.CommandTemplate, &rule.DeleteTemplate,
		(*pq.StringArray)(&rule.Errors), &rule.CompletedRun)
	return r.Scan(other...)
}
