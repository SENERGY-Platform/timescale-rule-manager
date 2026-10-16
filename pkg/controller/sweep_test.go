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

package controller

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"

	"github.com/SENERGY-Platform/models/go/models"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/model"
)

func shortTestUserId(t *testing.T, i *impl) (userId string, shortUserId string) {
	t.Helper()
	users, err := i.oidClient.GetUsers()
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		if user.Username == "testuser" {
			userId = user.Id
		}
	}
	if userId == "" {
		t.Fatal("testuser does not exist")
	}
	shortUserId, err = models.ShortenId(userId)
	if err != nil {
		t.Fatal(err)
	}
	return userId, shortUserId
}

func execInTx(t *testing.T, i *impl, f func(tx *sql.Tx) error) {
	t.Helper()
	tx, cancel, err := i.db.GetTx()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if err = f(tx); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func execStatements(t *testing.T, i *impl, statements ...string) {
	t.Helper()
	execInTx(t, i, func(tx *sql.Tx) error {
		for _, s := range statements {
			if _, err := tx.Exec(s); err != nil {
				return err
			}
		}
		return nil
	})
}

func errorsWithPrefix(errs []string, prefix string) (n int) {
	for _, e := range errs {
		if strings.HasPrefix(e, prefix) {
			n++
		}
	}
	return n
}

func TestWithoutTableErrors(t *testing.T) {
	errs := []string{"a: first", "a_ld: other table", "a:no space", "b: other", "a: second", "unprefixed", "a: "}
	got := withoutTableErrors(errs, "a")
	want := []string{"a_ld: other table", "a:no space", "b: other", "unprefixed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if errs[0] != "a: first" {
		t.Fatal("input slice was modified")
	}
	if got := withoutTableErrors(nil, "a"); got == nil || len(got) != 0 {
		t.Fatalf("got %#v for nil input", got)
	}
}

// Both cases share one compose stack: the package already runs close to go test's default ten-minute timeout.
func TestRuleSweep(t *testing.T) {
	_, _, _, c, _, _, _, cleanup := setup(t)
	defer cleanup()
	i := c.(*impl)
	userId, shortUserId := shortTestUserId(t, i)
	t.Run("rule errors keep one entry per table", func(t *testing.T) {
		testRuleErrorsKeepOneEntryPerTable(t, i, userId, shortUserId)
	})
	t.Run("ApplyAllRules isolates tables", func(t *testing.T) {
		testApplyAllRulesIsolatesTables(t, i, userId, shortUserId)
	})
}

func testRuleErrorsKeepOneEntryPerTable(t *testing.T, i *impl, userId string, shortUserId string) {
	tableA := "userid:" + shortUserId + "_export:F_gsbPBvSb6xEz8lAWpguw"
	tableB := "userid:" + shortUserId + "_export:7IUxe2sUT32dRXAZhzXczw"
	execStatements(t, i,
		`CREATE TABLE "`+tableA+`" (time TIMESTAMPTZ, val integer);`,
		`CREATE TABLE "`+tableB+`" (time TIMESTAMPTZ, val integer);`,
	)
	keptForeign := tableA + "_ld: entry of a table whose name starts like A"
	keptUnprefixed := "entry without table"
	rule := &model.Rule{
		Id:              "errors-per-table",
		Group:           "g",
		TableRegEx:      "^userid:" + shortUserId + "_export:",
		Users:           []string{userId},
		CommandTemplate: `SELECT * FROM "{{.Table}}_d";`, // fails until the table's _d companion exists
		Errors:          []string{keptForeign, keptUnprefixed},
	}
	execInTx(t, i, func(tx *sql.Tx) error { return i.db.InsertRule(rule, tx) })
	t.Cleanup(func() {
		execInTx(t, i, func(tx *sql.Tx) error { return i.db.DeleteRule(rule.Id, tx) })
	})

	apply := func(table string) {
		t.Helper()
		if _, err := i.ApplyAllRulesForTable(table, false); err != nil {
			t.Fatal(err)
		}
	}
	currentErrors := func() []string {
		t.Helper()
		r, _, err := i.GetRule(rule.Id)
		if err != nil {
			t.Fatal(err)
		}
		return r.Errors
	}
	assertKept := func(errs []string) {
		t.Helper()
		if errorsWithPrefix(errs, keptForeign) != 1 || errorsWithPrefix(errs, keptUnprefixed) != 1 {
			t.Fatalf("entries of other tables were touched: %#v", errs)
		}
	}

	apply(tableA)
	apply(tableA)
	apply(tableB)
	errs := currentErrors()
	if n := errorsWithPrefix(errs, tableA+": "); n != 1 {
		t.Fatalf("%d entries for table A after two failures, want 1: %#v", n, errs)
	}
	if n := errorsWithPrefix(errs, tableB+": "); n != 1 {
		t.Fatalf("%d entries for table B, want 1: %#v", n, errs)
	}
	assertKept(errs)
	if len(errs) != 4 {
		t.Fatalf("unexpected entries: %#v", errs)
	}

	execStatements(t, i, `CREATE TABLE "`+tableA+`_d" (v integer);`)
	apply(tableA)
	errs = currentErrors()
	if n := errorsWithPrefix(errs, tableA+": "); n != 0 {
		t.Fatalf("entry for table A kept after a success: %#v", errs)
	}
	if n := errorsWithPrefix(errs, tableB+": "); n != 1 {
		t.Fatalf("entry for table B lost: %#v", errs)
	}
	assertKept(errs)
	if len(errs) != 3 {
		t.Fatalf("unexpected entries: %#v", errs)
	}
}

func testApplyAllRulesIsolatesTables(t *testing.T, i *impl, userId string, shortUserId string) {
	good := "userid:" + shortUserId + "_export:cEL1di0o97qVfsf1bcHCTw"
	bad := "userid:" + shortUserId + "_export:mDrcbGbpQuuDlh9CURj33Q"
	execStatements(t, i,
		`CREATE TABLE "`+good+`" (time TIMESTAMPTZ, val integer);`,
		`CREATE TABLE "`+bad+`" (time TIMESTAMPTZ, val integer);`,
		`CREATE TABLE sweep_marker (tbl text);`,
	)
	rules := []*model.Rule{{
		Id:              "good",
		Group:           "good",
		TableRegEx:      "^" + good + "$",
		Users:           []string{userId},
		CommandTemplate: `INSERT INTO sweep_marker VALUES ('{{.Table}}');`,
	}, {
		Id:         "bad",
		Group:      "bad",
		TableRegEx: "^" + bad + "$",
		Users:      []string{userId},
		// Drops the rule savepoint before failing, so the table's transaction ends up aborted.
		CommandTemplate: `RELEASE SAVEPOINT rule; SELECT 1/0;`,
	}}
	execInTx(t, i, func(tx *sql.Tx) error {
		for _, r := range rules {
			if err := i.db.InsertRule(r, tx); err != nil {
				return err
			}
		}
		return nil
	})

	if err := i.ApplyAllRules(); err != nil {
		t.Fatal(err)
	}
	var n int
	execInTx(t, i, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM sweep_marker WHERE tbl = $1;`, good).Scan(&n)
	})
	if n != 1 {
		t.Fatalf("work on the good table was not committed: %d marker rows", n)
	}
}
