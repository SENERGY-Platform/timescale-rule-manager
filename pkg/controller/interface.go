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
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/model"
)

type Controller interface {
	CreateRule(rule *model.Rule) (res *model.TypedRule, code int, err error)
	UpdateRule(rule *model.Rule) (code int, err error)
	DeleteRule(id string) (code int, err error)
	GetRule(id string) (rule *model.TypedRule, code int, err error)
	ListRules(limit, offset int) (rules []model.TypedRule, code int, err error)

	ApplyAllRules() (err error)
	ApplyAllRulesForTable(table string, useDeleteTemplateInstead bool) (code int, err error)
}
