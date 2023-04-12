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
	"github.com/senergy-platform/timescale-rule-manager/pkg/model"
	"reflect"
	"testing"
)

func TestFillInsertQuery(t *testing.T) {
	fields, values := getFieldsAndValues(&model.Rule{
		Id:              "0",
		Description:     "test",
		Priority:        1,
		Group:           "2",
		TableRegEx:      ".*",
		Users:           []string{"sepl", "jürgen"},
		Roles:           []string{"user", "admin"},
		CommandTemplate: "CREATE TABLE wtf;",
		DeleteTemplate:  "DROP TABLE wtf;",
		Errors:          []string{},
		CompletedRun:    false,
	})
	if !reflect.DeepEqual(fields, []string{"Id", "Description", "Priority", "Group", "TableRegEx", "Users", "Roles", "CommandTemplate", "DeleteTemplate", "Errors", "CompletedRun"}) {
		t.Error("fields not as expected")
	}
	if !reflect.DeepEqual(values, []string{"'0'", "'test'", "'1'", "'2'", "'.*'", "'{\"sepl\", \"jürgen\"}'", "'{\"user\", \"admin\"}'", "'CREATE TABLE wtf;'", "'DROP TABLE wtf;'", "'{}'", "FALSE"}) {
		t.Error("values not as expected")
	}
}

func BenchmarkFillInsertQuery(b *testing.B) {
	rule := &model.Rule{
		Id:              "0",
		Description:     "test",
		Priority:        1,
		Group:           "2",
		TableRegEx:      ".*",
		Users:           []string{"sepl", "jürgen"},
		Roles:           []string{"user", "admin"},
		CommandTemplate: "CREATE TABLE wtf;",
		DeleteTemplate:  "DROP TABLE wtf;",
		CompletedRun:    false,
	}
	for i := 0; i < b.N; i++ {
		getFieldsAndValues(rule)
	}
}
