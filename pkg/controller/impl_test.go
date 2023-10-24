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
	"context"
	"fmt"
	"github.com/SENERGY-Platform/models/go/models"
	perm "github.com/SENERGY-Platform/permission-search/lib/client"
	perm_model "github.com/SENERGY-Platform/permission-search/lib/model"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/config"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/database"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/model"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/templates"
	"github.com/hashicorp/go-uuid"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestIntegration(t *testing.T) {
	_, _, conf, c, _, _, cleanup := setup(t)
	_, err := templates.New(&conf)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	rule := &model.Rule{
		Id:              "0",
		Description:     "test",
		Priority:        1,
		Group:           "2",
		TableRegEx:      "device",
		Users:           []string{"sepl", "jürgen"},
		Roles:           []string{"user", "admin"},
		CommandTemplate: "CREATE TABLE IF NOT EXISTS wtf;",
		DeleteTemplate:  "DROP TABLE wtf;",
		Errors:          []string{},
	}
	t.Run("Create", func(t *testing.T) {
		_, _, err := c.CreateRule(rule)
		if err == nil {
			t.Fatal("was able to set id myself")
		}
		rule.Id = ""
		typedRule, _, err := c.CreateRule(rule)
		if err != nil {
			t.Fatal(err)
		}
		rule.Id = typedRule.Id
		savedRule, _, err := c.GetRule(typedRule.Id)
		if err != nil {
			t.Fatal(err)
		}
		typed, err := rule.Type()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(*savedRule, *typed) {
			t.Fatal("Created != Read")
		}
	})
	t.Run("List", func(t *testing.T) {
		list, _, err := c.ListRules(100, 0)
		if err != nil {
			t.Fatal(err)
		}
		typed, err := rule.Type()
		if err != nil {
			t.Fatal(err)
		}
		l2 := []model.TypedRule{*typed}
		if !reflect.DeepEqual(list, l2) {
			t.Fatal("Created != Read")
		}
	})

	t.Run("FindMatchingTables", func(t *testing.T) {
		i := c.(*impl)
		tx, cancel, err := i.db.GetTx()
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		_, err = tx.Exec("CREATE TABLE IF NOT EXISTS \"device:7IUxe2sUT32dRXAZhzXczw_service:F_gsbPBvSb6xEz8lAWpguw\" (time TIMESTAMPTZ, val1 text, val2 integer);")
		if err != nil {
			t.Fatal(err)
		}
		tables, err := i.db.FindMatchingTables([]string{rule.Id}, tx)
		if err != nil {
			t.Fatal(err)
		}
		if len(tables) != 1 {
			t.Fatal("Unexpected number of matches found")
		}
	})

	t.Run("FindMatchingRules", func(t *testing.T) {
		i := c.(*impl)
		tx, cancel, err := i.db.GetTx()
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		table := "device:7IUxe2sUT32dRXAZhzXczw_service:F_gsbPBvSb6xEz8lAWpguw"
		_, err = tx.Exec("CREATE TABLE IF NOT EXISTS \"" + table + "\" (time TIMESTAMPTZ, val1 text, val2 integer);")
		if err != nil {
			t.Fatal(err)
		}
		rules, err := i.db.FindMatchingRules([]string{table}, tx)
		if err != nil {
			t.Fatal(err)
		}
		if len(rules) != 1 {
			t.Fatal("Unexpected number of matches found")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		_, err := c.DeleteRule(rule.Id)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = c.GetRule(rule.Id)
		if err == nil {
			t.Fatal("Not actually deleted")
		}
	})
}

func TestRuleLogicForDeviceTables(t *testing.T) {
	_, _, _, c, db, permSearch, cleanup := setup(t)
	i := c.(*impl)
	defer cleanup()
	users, err := i.oidClient.GetUsers()
	if err != nil {
		t.Fatal(err)
	}
	userId := ""
	for _, user := range users {
		if user.Username == "testuser" {
			userId = user.Id
			break
		}
	}
	if len(userId) == 0 {
		t.Fatal("testuser does not exist")
	}
	t.Run("Command & Delete Template", func(t *testing.T) {
		// ec85317b-6b14-4f7d-9d45-70198735dccf <-> 7IUxe2sUT32dRXAZhzXczw
		// 17f82c6c-f06f-49be-b113-3f25016a60bb <-> F_gsbPBvSb6xEz8lAWpguw
		// d6e5a728-c8c3-4473-b368-f514c11d48df <-> 1uWnKMjDRHOzaPUUwR1I3w
		permSearch.SetRights("devices", "ec85317b-6b14-4f7d-9d45-70198735dccf", perm_model.ResourceRights{
			ResourceRightsBase: perm_model.ResourceRightsBase{
				UserRights: map[string]perm_model.Right{
					userId: {
						Read:         true,
						Write:        true,
						Execute:      true,
						Administrate: true,
					},
				},
				GroupRights: map[string]perm_model.Right{
					"admin": {
						Read:         true,
						Write:        true,
						Execute:      true,
						Administrate: true,
					},
				},
			},
		})
		permSearch.SetRights("devices", "17f82c6c-f06f-49be-b113-3f25016a60bb", perm_model.ResourceRights{
			ResourceRightsBase: perm_model.ResourceRightsBase{
				UserRights: map[string]perm_model.Right{},
				GroupRights: map[string]perm_model.Right{
					"admin": {
						Read:         true,
						Write:        true,
						Execute:      true,
						Administrate: true,
					},
				},
			},
		})
		tx, cancel, err := db.GetTx()
		defer cancel()
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("CREATE TABLE IF NOT EXISTS \"device:7IUxe2sUT32dRXAZhzXczw_service:F_gsbPBvSb6xEz8lAWpguw\" (time TIMESTAMPTZ, val1 text, val2 integer);", tx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("SELECT create_hypertable('\"device:7IUxe2sUT32dRXAZhzXczw_service:F_gsbPBvSb6xEz8lAWpguw\"', 'time', if_not_exists => TRUE);", tx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("CREATE TABLE IF NOT EXISTS \"device:F_gsbPBvSb6xEz8lAWpguw_service:7IUxe2sUT32dRXAZhzXczw\" (time TIMESTAMPTZ, val1 text, val2 integer);", tx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			tx, cancel, err := i.db.GetTx()
			defer cancel()
			if err != nil {
				t.Fatal(err)
			}
			_, err = i.db.Exec("DROP TABLE \"device:7IUxe2sUT32dRXAZhzXczw_service:F_gsbPBvSb6xEz8lAWpguw\" CASCADE;", tx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = i.db.Exec("DROP TABLE \"device:F_gsbPBvSb6xEz8lAWpguw_service:7IUxe2sUT32dRXAZhzXczw\" CASCADE;", tx)
			if err != nil {
				t.Fatal(err)
			}
			err = tx.Commit()
			if err != nil {
				t.Fatal(err)
			}
		}()
		err = tx.Commit()
		if err != nil {
			t.Fatal(err)
		}

		rule := model.Rule{
			Priority:   0,
			Group:      "g",
			TableRegEx: "device.{23}_service.{23}",
			Users:      []string{userId},
			Roles:      nil,
			CommandTemplate: `
			CREATE MATERIALIZED VIEW IF NOT EXISTS "{{.Table}}_ld"
			WITH (timescaledb.continuous) AS
			SELECT                            
			  time_bucket(INTERVAL '1 day', time) AS time,
			 {{range $i, $el := slice .Columns 1}}{{if $i}},{{end}} last({{.}}, time) AS {{.}}{{end}}
			FROM "{{.Table}}"
			GROUP BY time_bucket(INTERVAL '1 day', time)
			WITH NO DATA;`,

			DeleteTemplate: "DROP MATERIALIZED VIEW \"{{.Table}}_ld\";",
		}

		typedRule, _, err := c.CreateRule(&rule)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Second) // rule logic applied async
		t.Run("Rule template executed for table correctly", func(t *testing.T) {
			columns, err := db.GetColumns("device:7IUxe2sUT32dRXAZhzXczw_service:F_gsbPBvSb6xEz8lAWpguw_ld")
			if err != nil {
				t.Fatal(err)
			}
			if len(columns) == 0 {
				t.Fatal("No Columns created")
			}
		})

		t.Run("Rule template ignored for table correctly", func(t *testing.T) {
			columns, err := db.GetColumns("device:F_gsbPBvSb6xEz8lAWpguw_service:7IUxe2sUT32dRXAZhzXczw_ld")
			if err != nil {
				t.Fatal(err)
			}
			if len(columns) != 0 {
				t.Fatal("Columns created")
			}
		})

		t.Run("Rule delete template executed for table correctly", func(t *testing.T) {
			_, err = c.DeleteRule(typedRule.Id)
			if err != nil {
				t.Fatal(err)
			}
			columns, err := db.GetColumns("device:7IUxe2sUT32dRXAZhzXczw_service:F_gsbPBvSb6xEz8lAWpguw_ld")
			if err != nil {
				t.Fatal(err)
			}
			if len(columns) != 0 {
				t.Fatal("No Columns deleted")
			}
		})
	})

}

func TestRuleLogicForExportTables(t *testing.T) {
	_, _, _, c, db, _, cleanup := setup(t)
	defer cleanup()
	i := c.(*impl)
	users, err := i.oidClient.GetUsers()
	if err != nil {
		t.Fatal(err)
	}
	userId := ""
	userId2 := ""
	for _, user := range users {
		if user.Username == "testuser" {
			userId = user.Id
		}
		if user.Username == "testuser2" {
			userId2 = user.Id
		}
	}
	if len(userId) == 0 || len(userId2) == 0 {
		t.Fatal("testuser or testuser2 does not exist")
	}
	shortUserId, err := models.ShortenId(userId)
	if err != nil {
		t.Fatal(err)
	}
	shortUserId2, err := models.ShortenId(userId2)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Command & Delete Template", func(t *testing.T) {
		// ec85317b-6b14-4f7d-9d45-70198735dccf <-> 7IUxe2sUT32dRXAZhzXczw
		// 17f82c6c-f06f-49be-b113-3f25016a60bb <-> F_gsbPBvSb6xEz8lAWpguw
		// d6e5a728-c8c3-4473-b368-f514c11d48df <-> 1uWnKMjDRHOzaPUUwR1I3w
		tx, cancel, err := db.GetTx()
		defer cancel()
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("CREATE TABLE IF NOT EXISTS \"userid:"+shortUserId+"_export:F_gsbPBvSb6xEz8lAWpguw\" (time TIMESTAMPTZ, val1 text, val2 integer);", tx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("SELECT create_hypertable('\"userid:"+shortUserId+"_export:F_gsbPBvSb6xEz8lAWpguw\"', 'time', if_not_exists => TRUE);", tx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("CREATE TABLE IF NOT EXISTS \"userid:"+shortUserId2+"_export:7IUxe2sUT32dRXAZhzXczw\" (time TIMESTAMPTZ, val1 text, val2 integer);", tx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			tx, cancel, err := db.GetTx()
			defer cancel()
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec("DROP TABLE \"userid:"+shortUserId+"_export:F_gsbPBvSb6xEz8lAWpguw\" CASCADE;", tx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec("DROP TABLE \"userid:"+shortUserId2+"_export:7IUxe2sUT32dRXAZhzXczw\" CASCADE;", tx)
			if err != nil {
				t.Fatal(err)
			}
			err = tx.Commit()
			if err != nil {
				t.Fatal(err)
			}
		}()
		err = tx.Commit()
		if err != nil {
			t.Fatal(err)
		}

		rule := model.Rule{
			Priority:   0,
			Group:      "g",
			TableRegEx: "userid.{23}_export.{23}",
			Users:      []string{userId},
			Roles:      nil,
			CommandTemplate: `
			CREATE MATERIALIZED VIEW IF NOT EXISTS "{{.Table}}_ld"
			WITH (timescaledb.continuous) AS
			SELECT                            
			  time_bucket(INTERVAL '1 day', time) AS time,
			 {{range $i, $el := slice .Columns 1}}{{if $i}},{{end}} last({{.}}, time) AS {{.}}{{end}}
			FROM "{{.Table}}"
			GROUP BY time_bucket(INTERVAL '1 day', time)
			WITH NO DATA;`,
			DeleteTemplate: "DROP MATERIALIZED VIEW \"{{.Table}}_ld\";",
		}

		typedRule, _, err := c.CreateRule(&rule)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Second) // rule logic applied async
		t.Run("Rule template executed for table correctly", func(t *testing.T) {
			columns, err := db.GetColumns("userid:" + shortUserId + "_export:F_gsbPBvSb6xEz8lAWpguw_ld")
			if err != nil {
				t.Fatal(err)
			}
			if len(columns) == 0 {
				t.Fatal("No Columns created")
			}
		})

		t.Run("Rule template ignored for table correctly", func(t *testing.T) {
			columns, err := db.GetColumns("userid:" + shortUserId2 + "_export:7IUxe2sUT32dRXAZhzXczw_ld")
			if err != nil {
				t.Fatal(err)
			}
			if len(columns) != 0 {
				t.Fatal("Columns created")
			}
		})

		t.Run("Rule delete template executed for table correctly", func(t *testing.T) {
			_, err = c.DeleteRule(typedRule.Id)
			if err != nil {
				t.Fatal(err)
			}
			columns, err := db.GetColumns("userid:" + shortUserId + "_export:F_gsbPBvSb6xEz8lAWpguw_ld")
			if err != nil {
				t.Fatal(err)
			}
			if len(columns) != 0 {
				t.Fatal("No Columns deleted")
			}
		})
	})

}

func TestUpdateErrorHandling(t *testing.T) {
	_, _, _, c, db, permSearch, cleanup := setup(t)
	i := c.(*impl)
	defer cleanup()
	users, err := i.oidClient.GetUsers()
	if err != nil {
		t.Fatal(err)
	}
	userId := ""
	for _, user := range users {
		if user.Username == "testuser" {
			userId = user.Id
			break
		}
	}
	if len(userId) == 0 {
		t.Fatal("testuser does not exist")
	}

	rule := &model.Rule{
		Priority:   0,
		Group:      "g",
		TableRegEx: "device.{23}_service.{23}",
		Users:      []string{userId},
		Roles:      nil,
		CommandTemplate: `
			CREATE MATERIALIZED VIEW IF NOT EXISTS "{{.Table}}_ld"
			WITH (timescaledb.continuous) AS
			SELECT                            
			  time_bucket(INTERVAL '1 day', time) AS time,
			 {{range $i, $el := slice .Columns 1}}{{if $i}},{{end}} last({{.}}, time) AS {{.}}{{end}}
			FROM "{{.Table}}"
			GROUP BY time_bucket(INTERVAL '1 day', time);`,
		DeleteTemplate: "DROP MATERIALIZED VIEW \"{{.Table}}_ld\";",
	}

	typedRule, _, err := c.CreateRule(rule)
	if err != nil {
		t.Fatal(err)
	}

	permSearch.SetRights("devices", "ec85317b-6b14-4f7d-9d45-70198735dccf", perm_model.ResourceRights{
		ResourceRightsBase: perm_model.ResourceRightsBase{
			UserRights: map[string]perm_model.Right{
				userId: {
					Read:         true,
					Write:        true,
					Execute:      true,
					Administrate: true,
				},
			},
		},
	})
	tx, cancel, err := db.GetTx()
	defer cancel()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS \"device:7IUxe2sUT32dRXAZhzXczw_service:F_gsbPBvSb6xEz8lAWpguw\" (time TIMESTAMPTZ, val1 text, val2 integer);", tx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("SELECT create_hypertable('\"device:7IUxe2sUT32dRXAZhzXczw_service:F_gsbPBvSb6xEz8lAWpguw\"', 'time', if_not_exists => TRUE);", tx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		tx, cancel, err := db.GetTx()
		defer cancel()
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("DROP TABLE \"device:7IUxe2sUT32dRXAZhzXczw_service:F_gsbPBvSb6xEz8lAWpguw\" CASCADE;", tx)
		if err != nil {
			t.Fatal(err)
		}
		err = tx.Commit()
		if err != nil {
			t.Fatal(err)
		}
	}()
	err = tx.Commit()
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.UpdateRule(typedRule.Rule)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second) // rule logic applied async
	r, _, err := c.GetRule(typedRule.Id)
	if err != nil {
		t.Fatal(err)
	}
	if r.CompletedRun || r.Errors == nil || len(r.Errors) == 0 {
		t.Fatal("expected error")
	}
	rule.CommandTemplate = `
			CREATE MATERIALIZED VIEW IF NOT EXISTS "{{.Table}}_ld"
			WITH (timescaledb.continuous) AS
			SELECT                            
			  time_bucket(INTERVAL '1 day', time) AS time,
			 {{range $i, $el := slice .Columns 1}}{{if $i}},{{end}} last({{.}}, time) AS {{.}}{{end}}
			FROM "{{.Table}}"
			GROUP BY time_bucket(INTERVAL '1 day', time) WITH NO DATA;`
	_, err = c.UpdateRule(typedRule.Rule)
	if err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T) (ctx context.Context, wg *sync.WaitGroup, conf config.Config, c Controller, db database.DB, permSearch *perm.TestClient, cleanup func()) {
	ctx = context.Background()
	wg = &sync.WaitGroup{}
	permSearch = perm.NewTestClient()
	var err error
	conf = config.Config{
		KafkaBootstrap:              "localhost:9092",
		KafkaTopicTableUpdates:      "timescale-table-updates",
		KafkaTopicPermissionUpdates: "permissions_done",
		KafkaOffset:                 "earliest",
		KafkaGroupId:                "timescale-rule-manager",
		PostgresHost:                "localhost",
		PostgresPort:                5432,
		PostgresUser:                "username",
		PostgresPw:                  "password",
		PostgresDb:                  "database",
		PostgresRuleSchema:          "public",
		PostgresRuleTable:           "rules",
		KeycloakUrl:                 "http://localhost:8123",
		KeycloakClientId:            "myapp",
		KeycloakClientSecret:        "d0b8122f-8dfb-46b7-b68a-f5cc4e25d000",
		ApplyRulesAtStartup:         false,
		Timeout:                     "30s",
		Debug:                       true,
	}
	config.HandleEnvironmentVars(&conf)
	t.Run("Setup DB", func(t *testing.T) {
		db, err = database.New(conf.PostgresHost, conf.PostgresPort, conf.PostgresUser, conf.PostgresPw, conf.PostgresDb, conf.PostgresRuleSchema, conf.PostgresRuleTable, conf.Timeout, conf.Debug, ctx, wg)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Test Dependency Connection", func(t *testing.T) {
		c, err = New(conf, db, permSearch, ctx, wg)
		if err != nil {
			t.Fatal(err.Error() + " Did you launch using test.sh?")
		}
	})
	return ctx, wg, conf, c, db, permSearch, func() {
		tx, cancel, err := db.GetTx()
		if err != nil {
			t.Fatal(err)
		}
		defer cancel()
		_, err = db.Exec("DROP TABLE "+conf.PostgresRuleSchema+"."+conf.PostgresRuleTable+";", tx)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestShortIds(t *testing.T) {
	// this is just a utility to generate id pairs for other tests
	long, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	short, err := models.ShortenId(long)
	if err != nil {
		t.Fatal(err)
	}
	long, err = models.LongId(short)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(long + " <-> " + short)
}
