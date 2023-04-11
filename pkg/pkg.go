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
	"github.com/SENERGY-Platform/permission-search/lib/client"
	"github.com/senergy-platform/timescale-rule-manager/pkg/api"
	"github.com/senergy-platform/timescale-rule-manager/pkg/config"
	"github.com/senergy-platform/timescale-rule-manager/pkg/controller"
	"github.com/senergy-platform/timescale-rule-manager/pkg/database"
	"sync"
)

func Start(ctx context.Context, conf config.Config) (wg *sync.WaitGroup, err error) {
	wg = &sync.WaitGroup{}

	db, err := database.New(conf.PostgresHost, conf.PostgresPort, conf.PostgresUser, conf.PostgresPw, conf.PostgresDb, conf.PostgresRuleSchema, conf.PostgresRuleTable, conf.Debug, ctx, wg)
	if err != nil {
		return wg, err
	}

	permissionSearch := client.NewClient(conf.PermissionSearchUrl)

	control, err := controller.New(conf, db, permissionSearch, ctx, wg)
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
