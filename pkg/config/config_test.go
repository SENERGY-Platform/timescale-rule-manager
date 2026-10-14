/*
 *    Copyright (c) 2026 InfAI (CC SES)
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

package config

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

func TestHandleEnvironmentVarsKeepsSecretsOutOfTheLog(t *testing.T) {
	secrets := map[string]string{
		"POSTGRES_USER":          "the-user",
		"POSTGRES_PW":            "the-password",
		"KEYCLOAK_CLIENT_SECRET": "the-client-secret",
	}
	for k, v := range secrets {
		t.Setenv(k, v)
	}
	t.Setenv("KEYCLOAK_CLIENT_ID", "the-client-id")

	var out bytes.Buffer
	log.SetOutput(&out)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	config := &Config{}
	HandleEnvironmentVars(config)

	for k, v := range secrets {
		if strings.Contains(out.String(), v) {
			t.Errorf("%v logged: %q", k, out.String())
		}
	}
	if !strings.Contains(out.String(), "the-client-id") {
		t.Errorf("other values not logged: %q", out.String())
	}
	if config.KeycloakClientSecret != "the-client-secret" {
		t.Errorf("secret not applied: %q", config.KeycloakClientSecret)
	}
}
