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

package model

type Rule struct {
	Id              string   `sqltype:"text" sqlextra:"primary key" json:"id,omitempty"` // Set by API
	Description     string   `sqltype:"text" json:"description,omitempty"`
	Priority        int      `sqltype:"integer" json:"priority"`
	Group           string   `sqltype:"text" json:"group,omitempty"`
	TableRegEx      string   `sqltype:"text" json:"table_reg_ex,omitempty"`
	Users           []string `sqltype:"text[]" json:"users,omitempty"`
	Roles           []string `sqltype:"text[]" json:"roles,omitempty"`
	CommandTemplate string   `sqltype:"text" json:"command_template,omitempty"`
	DeleteTemplate  string   `sqltype:"text" json:"delete_template,omitempty"`
	Errors          []string `sqltype:"text[]" json:"errors,omitempty"`
	CompletedRun    bool     `sqltype:"boolean" sqlextra:"not null default false" json:"completed_run"`
}

type TypedRule struct {
	*Rule
	Type     RuleType           `json:"type"`
	Template string             `json:"template"`
	Target   TemplateRuleTarget `json:"target,omitempty"`
}

type RuleType = string

const RuleTypeCustom = "custom"
const RuleTypeTemplate = "template"

type TemplateRuleTarget = string

const TemplateRuleTargetDevice TemplateRuleTarget = "device"
const TemplateRuleTargetExport TemplateRuleTarget = "export"

type TemplateRule struct {
	Id       string             `json:"id,omitempty"` // Set by API
	Target   TemplateRuleTarget `json:"target,omitempty"`
	Users    []string           `json:"users,omitempty"`
	Roles    []string           `json:"roles,omitempty"`
	Template string             `json:"template,omitempty"`
}

type TableInfo struct {
	Table          string
	UserIds        []string
	Roles          []string
	ShortUserId    string
	DeviceId       string
	ShortDeviceId  string
	ServiceId      string
	ShortServiceId string
	ExportId       string
	ShortExportId  string
	Columns        []string
	Timezone       string
}
