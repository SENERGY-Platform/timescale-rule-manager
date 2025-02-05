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
	"encoding/json"
	"github.com/SENERGY-Platform/models/go/models"
	perm "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	model2 "github.com/SENERGY-Platform/permissions-v2/pkg/model"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/model"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/templates"
	"testing"
	"time"
)

func TestKafkaUpdateBehaviour(t *testing.T) {
	_, _, conf, c, db, permV2, _, cleanup := setup(t)
	_, err := templates.New(&conf)
	if err != nil {
		t.Fatal(err)
	}
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
	if len(userId) == 0 {
		t.Fatal("testuser does not exist")
	}
	if len(userId2) == 0 {
		t.Fatal("testuser2 does not exist")
	}
	shortUserId, err := models.ShortenId(userId)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Update on "+conf.KafkaTopicPermissionUpdates, func(t *testing.T) {
		// 7042f576-2d28-f7ba-957e-c7f56dc1c24f <-> cEL1di0o97qVfsf1bcHCTw
		// 983adc6c-66e9-42eb-8396-1f425118f7dd <-> mDrcbGbpQuuDlh9CURj33Q

		_, err, _ = permV2.SetPermission(perm.InternalAdminToken, "devices", "7042f576-2d28-f7ba-957e-c7f56dc1c24f", perm.ResourcePermissions{
			UserPermissions: map[string]model2.PermissionsMap{
				userId2: {
					Read:         true,
					Write:        true,
					Execute:      true,
					Administrate: true,
				},
			},
			GroupPermissions: map[string]model2.PermissionsMap{},
			RolePermissions: map[string]model2.PermissionsMap{
				"admin": {
					Read:         true,
					Write:        true,
					Execute:      true,
					Administrate: true,
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		_, err, _ = permV2.SetPermission(perm.InternalAdminToken, "devices", "983adc6c-66e9-42eb-8396-1f425118f7dd", perm.ResourcePermissions{
			UserPermissions: map[string]model2.PermissionsMap{
				userId2: {
					Read:         true,
					Write:        true,
					Execute:      true,
					Administrate: true,
				},
			},
			GroupPermissions: map[string]model2.PermissionsMap{},
			RolePermissions: map[string]model2.PermissionsMap{
				"admin": {
					Read:         true,
					Write:        true,
					Execute:      true,
					Administrate: true,
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		tx, cancel, err := db.GetTx()
		defer cancel()
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("CREATE TABLE IF NOT EXISTS \"device:cEL1di0o97qVfsf1bcHCTw_service:mDrcbGbpQuuDlh9CURj33Q\" (time TIMESTAMPTZ, val1 text, val2 integer);", tx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("SELECT create_hypertable('\"device:cEL1di0o97qVfsf1bcHCTw_service:mDrcbGbpQuuDlh9CURj33Q\"', 'time', if_not_exists => TRUE);", tx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("CREATE TABLE IF NOT EXISTS \"device:mDrcbGbpQuuDlh9CURj33Q_service:cEL1di0o97qVfsf1bcHCTw\" (time TIMESTAMPTZ, val1 text, val2 integer);", tx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			tx, cancel, err := db.GetTx()
			defer cancel()
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec("DROP TABLE \"device:cEL1di0o97qVfsf1bcHCTw_service:mDrcbGbpQuuDlh9CURj33Q\" CASCADE;", tx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec("DROP TABLE \"device:mDrcbGbpQuuDlh9CURj33Q_service:cEL1di0o97qVfsf1bcHCTw\" CASCADE;", tx)
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
			GROUP BY 1
			WITH NO DATA;
			`,
			DeleteTemplate: "DROP MATERIALIZED VIEW \"{{.Table}}_ld\";",
		}

		_, _, err = c.CreateRule(&rule)
		if err != nil {
			t.Fatal(err)
		}

		_, err, _ = permV2.SetPermission(perm.InternalAdminToken, "devices", "7042f576-2d28-f7ba-957e-c7f56dc1c24f", perm.ResourcePermissions{
			UserPermissions: map[string]model2.PermissionsMap{
				userId: {
					Read:         true,
					Write:        true,
					Execute:      true,
					Administrate: true,
				},
			},
			GroupPermissions: map[string]model2.PermissionsMap{},
			RolePermissions: map[string]model2.PermissionsMap{
				"admin": {
					Read:         true,
					Write:        true,
					Execute:      true,
					Administrate: true,
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		updateMsg := model.PermissionSearchDoneMessage{
			ResourceKind: "devices",
			ResourceId:   "7042f576-2d28-f7ba-957e-c7f56dc1c24f",
			Handler:      model.DoneMessageHandlerDeviceRepo,
		}
		b, err := json.Marshal(updateMsg)
		if err != nil {
			t.Fatal(err)
		}
		err = i.kafkaMessageHandler(conf.KafkaTopicPermissionUpdates, b, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Second) // rule logic applied async
		t.Run("Rule template executed for table correctly", func(t *testing.T) {
			columns, err := db.GetColumns("device:cEL1di0o97qVfsf1bcHCTw_service:mDrcbGbpQuuDlh9CURj33Q_ld")
			if err != nil {
				t.Fatal(err)
			}
			if len(columns) == 0 {
				t.Fatal("No Columns created")
			}
		})

		t.Run("Rule template ignored for table correctly", func(t *testing.T) {
			columns, err := db.GetColumns("device:mDrcbGbpQuuDlh9CURj33Q_service:cEL1di0o97qVfsf1bcHCTw_ld")
			if err != nil {
				t.Fatal(err)
			}
			if len(columns) != 0 {
				t.Fatal("Columns created")
			}
		})
	})

	t.Run("Update on "+conf.KafkaTopicTableUpdates, func(t *testing.T) {
		// 58db2e81-aafd-1d8e-a7c7-9b17bfdd206b <-> WNsugar9HY6nx5sXv90gaw
		tx, cancel, err := db.GetTx()
		defer cancel()
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
			GROUP BY 1
			WITH NO DATA;`,
			DeleteTemplate: "DROP MATERIALIZED VIEW \"{{.Table}}_ld\";",
		}

		_, _, err = c.CreateRule(&rule)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("CREATE TABLE IF NOT EXISTS \"userid:"+shortUserId+"_export:WNsugar9HY6nx5sXv90gaw\" (time TIMESTAMPTZ, val1 text, val2 integer);", tx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec("SELECT create_hypertable('\"userid:"+shortUserId+"_export:WNsugar9HY6nx5sXv90gaw\"', 'time', if_not_exists => TRUE);", tx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			tx, cancel, err := db.GetTx()
			defer cancel()
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec("DROP TABLE \"userid:"+shortUserId+"_export:WNsugar9HY6nx5sXv90gaw\" CASCADE;", tx)
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
		updateMsg := model.TableEditMessage{
			Method: model.TableEditMessageMethodPut,
			Tables: []string{"userid:" + shortUserId + "_export:WNsugar9HY6nx5sXv90gaw"},
		}
		b, err := json.Marshal(updateMsg)
		if err != nil {
			t.Fatal(err)
		}
		err = i.kafkaMessageHandler(conf.KafkaTopicTableUpdates, b, time.Now())
		if err != nil {
			t.Fatal(err)
		}

		t.Run("Rule template executed for table correctly", func(t *testing.T) {
			columns, err := db.GetColumns("userid:" + shortUserId + "_export:WNsugar9HY6nx5sXv90gaw_ld")
			if err != nil {
				t.Fatal(err)
			}
			if len(columns) == 0 {
				t.Fatal("No Columns created")
			}
		})
	})
}
