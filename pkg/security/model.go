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

package security

import "time"

type OpenidToken struct {
	AccessToken      string    `json:"access_token"`
	ExpiresIn        float64   `json:"expires_in"`
	RefreshExpiresIn float64   `json:"refresh_expires_in"`
	RefreshToken     string    `json:"refresh_token"`
	TokenType        string    `json:"token_type"`
	RequestTime      time.Time `json:"-"`
}

func (this *OpenidToken) JwtToken() string {
	return "Bearer " + this.AccessToken
}

type RoleMappings struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Composite   bool   `json:"composite"`
	ClientRole  bool   `json:"clientRole"`
	ContainerId string `json:"containerId"`
}

type User struct {
	Id                         string        `json:"id"`
	CreatedTimestamp           int64         `json:"createdTimestamp"`
	Username                   string        `json:"username"`
	Enabled                    bool          `json:"enabled"`
	Totp                       bool          `json:"totp"`
	EmailVerified              bool          `json:"emailVerified"`
	DisableableCredentialTypes []interface{} `json:"disableableCredentialTypes"`
	RequiredActions            []interface{} `json:"requiredActions"`
	NotBefore                  int           `json:"notBefore"`
	Access                     struct {
		ManageGroupMembership bool `json:"manageGroupMembership"`
		View                  bool `json:"view"`
		MapRoles              bool `json:"mapRoles"`
		Impersonate           bool `json:"impersonate"`
		Manage                bool `json:"manage"`
	} `json:"access"`
}
