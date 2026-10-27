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
	"github.com/lib/pq"
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

func (this *impl) queryStrings(query string, tx queryable, args ...any) (result []string, err error) {
	rows, err := tx.Query(query, args...)
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

// getFieldsAndArgs returns the sqltype-tagged columns of rule and their values as query arguments.
func getFieldsAndArgs(rule *model.Rule) (fields []string, args []any) {
	fields = []string{}
	args = []any{}
	t := reflect.TypeOf(*rule)
	v := reflect.ValueOf(*rule)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		_, ok := field.Tag.Lookup("sqltype")
		if !ok {
			continue
		}
		fields = append(fields, field.Name)
		var arg any
		switch value := v.Field(i).Interface().(type) {
		case string, int, int64, float32, float64, bool:
			arg = value
		case []string:
			if value == nil {
				value = []string{} // stored as '{}' rather than NULL, as the literal-building code did
			}
			arg = pq.Array(value)
		case *bool:
			if value != nil {
				arg = *value
			}
		}
		args = append(args, arg)
	}
	return fields, args
}

// textArray binds values as a text[] parameter; nil becomes an empty array so "= ANY" and "&&" match nothing
// instead of comparing against NULL.
func textArray(values []string) any {
	if values == nil {
		values = []string{}
	}
	return pq.Array(values)
}

// escapeLike escapes the LIKE wildcards in s, so it matches only itself.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
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
