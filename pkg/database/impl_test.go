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
	"database/sql/driver"
	"reflect"
	"testing"

	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/model"
)

func TestFillInsertQuery(t *testing.T) {
	fields, args := getFieldsAndArgs(&model.Rule{
		Id:              "0",
		Description:     "test",
		Priority:        1,
		Group:           "2",
		TableRegEx:      ".*",
		Users:           []string{"user-a", "grün"},
		Roles:           []string{"user", "admin"},
		CommandTemplate: "CREATE TABLE wtf;",
		DeleteTemplate:  "DROP TABLE wtf;",
		Errors:          []string{`it's`, `say "hi"`, `back\slash`},
		CompletedRun:    false,
	})
	if !reflect.DeepEqual(fields, []string{"Id", "Description", "Priority", "Group", "TableRegEx", "Users", "Roles", "CommandTemplate", "DeleteTemplate", "Errors", "CompletedRun"}) {
		t.Error("fields not as expected")
	}
	values := make([]any, len(args))
	for i, arg := range args {
		if valuer, ok := arg.(driver.Valuer); ok {
			v, err := valuer.Value()
			if err != nil {
				t.Fatal(err)
			}
			values[i] = v
		} else {
			values[i] = arg
		}
	}
	want := []any{"0", "test", 1, "2", ".*", `{"user-a","grün"}`, `{"user","admin"}`, "CREATE TABLE wtf;", "DROP TABLE wtf;", `{"it's","say \"hi\"","back\\slash"}`, false}
	if !reflect.DeepEqual(values, want) {
		t.Errorf("values not as expected\n got %#v\nwant %#v", values, want)
	}
}

func TestFillInsertQueryNilSlices(t *testing.T) {
	_, args := getFieldsAndArgs(&model.Rule{})
	for _, i := range []int{5, 6, 9} {
		v, err := args[i].(driver.Valuer).Value()
		if err != nil {
			t.Fatal(err)
		}
		if v != "{}" {
			t.Errorf("arg %d: got %#v, want {}", i, v)
		}
	}
}

func BenchmarkFillInsertQuery(b *testing.B) {
	rule := &model.Rule{
		Id:              "0",
		Description:     "test",
		Priority:        1,
		Group:           "2",
		TableRegEx:      ".*",
		Users:           []string{"user-a", "grün"},
		Roles:           []string{"user", "admin"},
		CommandTemplate: "CREATE TABLE wtf;",
		DeleteTemplate:  "DROP TABLE wtf;",
		CompletedRun:    false,
	}
	for i := 0; i < b.N; i++ {
		getFieldsAndArgs(rule)
	}
}
