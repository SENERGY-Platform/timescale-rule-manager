/*
 * Copyright 2026 InfAI (CC SES)
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
	"errors"
	"reflect"
	"testing"

	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/log"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/model"
)

func inTx(t *testing.T, db *impl, f func(tx *sql.Tx) error) error {
	t.Helper()
	tx, cancel, err := db.GetTx()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	err = f(tx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func getRule(t *testing.T, db *impl, id string) *model.Rule {
	t.Helper()
	var rule *model.Rule
	err := inTx(t, db, func(tx *sql.Tx) (err error) {
		rule, err = db.GetRule(id, tx)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return rule
}

// Values the old hand-built literals mangled or rejected: quotes, backslashes and array syntax inside elements.
var adversarialTexts = []string{`it's`, `say "hi"`, `back\slash`, `\"'`, `{a,b}`, `,`, `NULL`, ``, `grün`}

func TestRuleQueries(t *testing.T) {
	log.InitForTest()
	server := startLockTestServer(t)
	db := server.newInstance(t).impl

	t.Run("insert and update keep quotes and backslashes", func(t *testing.T) {
		rule := &model.Rule{
			Id:              "round-trip",
			Description:     `a 'quoted' "description" with \ inside`,
			Priority:        7,
			Group:           `g'"\`,
			TableRegEx:      `^x'\d+$`,
			Users:           []string{"user-a", `user'b`},
			Roles:           []string{`role"c`},
			CommandTemplate: `SELECT '{{.Table}}';`,
			DeleteTemplate:  `SELECT E'\\';`,
			Errors:          adversarialTexts,
			CompletedRun:    true,
		}
		err := inTx(t, db, func(tx *sql.Tx) error { return db.InsertRule(rule, tx) })
		if err != nil {
			t.Fatal(err)
		}
		got := getRule(t, db, rule.Id)
		if !reflect.DeepEqual(got, rule) {
			t.Fatalf("after insert\n got %#v\nwant %#v", got, rule)
		}

		rule.Errors = append([]string{`table: relation "x'y\z" does not exist`}, adversarialTexts...)
		rule.Description = `\'`
		rule.CompletedRun = false
		err = inTx(t, db, func(tx *sql.Tx) error { return db.UpdateRule(rule, tx) })
		if err != nil {
			t.Fatal(err)
		}
		got = getRule(t, db, rule.Id)
		if !reflect.DeepEqual(got, rule) {
			t.Fatalf("after update\n got %#v\nwant %#v", got, rule)
		}
	})

	t.Run("nil slices are stored as empty arrays", func(t *testing.T) {
		rule := &model.Rule{Id: "nil-slices", TableRegEx: "^none$"}
		err := inTx(t, db, func(tx *sql.Tx) error { return db.InsertRule(rule, tx) })
		if err != nil {
			t.Fatal(err)
		}
		got := getRule(t, db, rule.Id)
		for name, s := range map[string][]string{"Users": got.Users, "Roles": got.Roles, "Errors": got.Errors} {
			if s == nil || len(s) != 0 {
				t.Errorf("%s: got %#v, want an empty non-nil slice", name, s)
			}
		}
	})

	t.Run("update of an id with a quote reports not found", func(t *testing.T) {
		rule := &model.Rule{Id: `missing' OR 'a'='a`, TableRegEx: "^none$"}
		err := inTx(t, db, func(tx *sql.Tx) error { return db.UpdateRule(rule, tx) })
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("FindMatchingTables returns each table once", func(t *testing.T) {
		rules := []*model.Rule{
			{Id: "match-1", TableRegEx: "^match_once_table$"},
			{Id: "match-2", TableRegEx: "match_once"},
		}
		err := inTx(t, db, func(tx *sql.Tx) error {
			if _, err := tx.Exec(`CREATE TABLE public.match_once_table (v integer);`); err != nil {
				return err
			}
			for _, r := range rules {
				if err := db.InsertRule(r, tx); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		var tables []string
		err = inTx(t, db, func(tx *sql.Tx) (err error) {
			tables, err = db.FindMatchingTables([]string{"match-1", "match-2"}, tx)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tables, []string{"match_once_table"}) {
			t.Fatalf("got %#v", tables)
		}
	})
}
