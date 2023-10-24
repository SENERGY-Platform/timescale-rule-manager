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

package model

func (rule *Rule) Copy() Rule {
	myRule := Rule{
		Id:          rule.Id,
		Description: rule.Description,
		Priority:    rule.Priority,
		Group:       rule.Group,
		TableRegEx:  rule.TableRegEx,
		//Users:           rule.Users,
		//Roles:           rule.Roles,
		CommandTemplate: rule.CommandTemplate,
		DeleteTemplate:  rule.DeleteTemplate,
		//Errors:          rule.Errors,
		CompletedRun: rule.CompletedRun,
	}
	if rule.Users != nil {
		myRule.Users = []string{}
		myRule.Users = append(myRule.Users, rule.Users...)
	}
	if rule.Roles != nil {
		myRule.Roles = []string{}
		myRule.Roles = append(myRule.Roles, rule.Roles...)
	}
	if rule.Errors != nil {
		myRule.Errors = []string{}
		myRule.Errors = append(myRule.Errors, rule.Errors...)
	}
	return myRule
}
