/*
 *    Copyright 2023 InfAI (CC SES)
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package pkg

import (
	"context"
	"log"
	"sync"

	deviceRepo "github.com/SENERGY-Platform/device-repository/lib/client"
	"github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/api"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/config"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/controller"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/database"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/templates"
)

func Start(fatal func(error), ctx context.Context, conf config.Config) (wg *sync.WaitGroup, err error) {
	wg = &sync.WaitGroup{}

	_, err = templates.New(&conf)
	if err != nil {
		log.Println("WARNING: Could not read templates: " + err.Error())
	}
	db, err := database.New(conf.PostgresHost, conf.PostgresPort, conf.PostgresUser, conf.PostgresPw, conf.PostgresDb, conf.PostgresRuleSchema, conf.PostgresRuleTable, conf.Timeout, conf.PostgresLockKey, conf.Debug, ctx, wg)
	if err != nil {
		return wg, err
	}

	permV2 := client.New(conf.PermissionsV2Url)

	deviceRepoClient := deviceRepo.NewClient(conf.DeviceRepoUrl, nil)

	control, err := controller.New(conf, db, permV2, deviceRepoClient, fatal, ctx, wg)
	if err != nil {
		return wg, err
	}

	if conf.ApplyRulesAtStartup {
		err = control.ApplyAllRules()
		if err != nil {
			return wg, err
		}
	}

	err = api.Start(ctx, wg, conf, control)
	return
}
