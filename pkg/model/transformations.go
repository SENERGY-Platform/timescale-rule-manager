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

import (
	"errors"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/templates"
)

func (rule *Rule) Type() (*TypedRule, error) {
	ts, err := templates.New(nil)
	if err != nil {
		return nil, err
	}
	for template, tmpl := range ts.Templates {
		if rule.matchesTemplate(tmpl) {
			tr := &TypedRule{
				Rule:     rule,
				Type:     RuleTypeTemplate,
				Template: template,
			}
			if rule.TableRegEx == "^device:.{22}_service:.{22}$" {
				tr.Target = TemplateRuleTargetDevice
			} else if rule.TableRegEx == "^userid:.{22}_export:.{22}$" {
				tr.Target = TemplateRuleTargetExport
			}
			return tr, nil
		}
	}
	return &TypedRule{
		Rule: rule,
		Type: RuleTypeCustom,
	}, nil
}

func (rule *Rule) matchesTemplate(template templates.Template) bool {
	return template.Group == rule.Group && template.CommandTemplate == rule.CommandTemplate && template.DeleteTemplate == rule.DeleteTemplate
}

func (r *TemplateRule) Rule() (*Rule, error) {
	ts, err := templates.New(nil)
	if err != nil {
		return nil, err
	}

	tmpl, ok := ts.Templates[r.Template]
	if !ok {
		return nil, errors.New("unknown TemplateRule template")
	}
	rule := Rule{
		Id:              r.Id,
		Description:     tmpl.Description,
		Priority:        tmpl.Priority,
		Group:           tmpl.Group,
		Users:           r.Users,
		Roles:           r.Roles,
		CommandTemplate: tmpl.CommandTemplate,
		DeleteTemplate:  tmpl.DeleteTemplate,
	}
	switch r.Target {
	case TemplateRuleTargetDevice:
		rule.TableRegEx = "^device:.{22}_service:.{22}$"
	case TemplateRuleTargetExport:
		rule.TableRegEx = "^userid:.{22}_export:.{22}$"
	default:
		return nil, errors.New("unknown TemplateRule target")
	}

	return &rule, nil
}
