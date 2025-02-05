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

package config

import (
	"encoding/json"
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	ApiPort string `json:"api_port"`

	KafkaBootstrap              string `json:"kafka_bootstrap"`
	KafkaTopicTableUpdates      string `json:"kafka_topic_table_updates"`
	KafkaTopicPermissionUpdates string `json:"kafka_topic_permission_updates"`
	KafkaOffset                 string `json:"kafka_offset"`
	KafkaGroupId                string `json:"kafka_group_id"`

	PostgresHost       string `json:"postgres_host"`
	PostgresPort       int    `json:"postgres_port"`
	PostgresUser       string `json:"postgres_user" config:"secret"`
	PostgresPw         string `json:"postgres_pw" config:"secret"`
	PostgresDb         string `json:"postgres_db"`
	PostgresRuleSchema string `json:"postgres_rule_schema"`
	PostgresRuleTable  string `json:"postgres_rule_table"`
	PostgresLockKey    int64  `json:"postgres_lock_key"`

	PermissionsV2Url string `json:"permissions_v2_url"`

	KeycloakUrl          string `json:"keycloak_url"`
	KeycloakClientId     string `json:"keycloak_client_id"`
	KeycloakClientSecret string `json:"keycloak_client_secret"`

	DeviceIdPrefix  string `json:"device_id_prefix"`
	ServiceIdPrefix string `json:"service_id_prefix"`

	ApplyRulesAtStartup bool   `json:"apply_rules_at_startup"`
	Timeout             string `json:"timeout"`
	TemplateDir         string `json:"template_dir"`
	Debug               bool   `json:"debug"`
	SlowMuxLock         string `json:"slow_mux_lock"`
	DefaultTimezone     string `json:"default_timezone"`
	DeviceRepoUrl       string `json:"device_repo_url"`
}

// loads config from json in location and used environment variables (e.g ZookeeperUrl --> ZOOKEEPER_URL)
func LoadConfig(location string) (config Config, err error) {
	file, err2 := os.Open(location)
	if err2 != nil {
		log.Println("error on config load: ", err2)
		return config, err2
	}
	decoder := json.NewDecoder(file)
	err2 = decoder.Decode(&config)
	if err2 != nil {
		log.Println("invalid config json: ", err2)
		return config, err2
	}
	HandleEnvironmentVars(&config)
	return config, nil
}

var camel = regexp.MustCompile("(^[^A-Z]*|[A-Z]*)([A-Z][^A-Z]+|$)")

func fieldNameToEnvName(s string) string {
	var a []string
	for _, sub := range camel.FindAllStringSubmatch(s, -1) {
		if sub[1] != "" {
			a = append(a, sub[1])
		}
		if sub[2] != "" {
			a = append(a, sub[2])
		}
	}
	return strings.ToUpper(strings.Join(a, "_"))
}

// preparations for docker
func HandleEnvironmentVars(config *Config) {
	configValue := reflect.Indirect(reflect.ValueOf(config))
	configType := configValue.Type()
	for index := 0; index < configType.NumField(); index++ {
		fieldName := configType.Field(index).Name
		fieldConfig := configType.Field(index).Tag.Get("config")
		envName := fieldNameToEnvName(fieldName)
		envValue := os.Getenv(envName)
		if envValue != "" {
			if !strings.Contains(fieldConfig, "secret") {
				log.Println("use environment variable: ", envName, " = ", envValue)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Int64 {
				i, _ := strconv.ParseInt(envValue, 10, 64)
				configValue.FieldByName(fieldName).SetInt(i)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Uint8 {
				i, _ := strconv.ParseUint(envValue, 10, 8)
				configValue.FieldByName(fieldName).SetUint(i)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.String {
				configValue.FieldByName(fieldName).SetString(envValue)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Bool {
				b, _ := strconv.ParseBool(envValue)
				configValue.FieldByName(fieldName).SetBool(b)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Float64 {
				f, _ := strconv.ParseFloat(envValue, 64)
				configValue.FieldByName(fieldName).SetFloat(f)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Slice {
				val := []string{}
				for _, element := range strings.Split(envValue, ",") {
					val = append(val, strings.TrimSpace(element))
				}
				configValue.FieldByName(fieldName).Set(reflect.ValueOf(val))
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Map {
				value := map[string]string{}
				for _, element := range strings.Split(envValue, ",") {
					keyVal := strings.Split(element, ":")
					key := strings.TrimSpace(keyVal[0])
					val := strings.TrimSpace(keyVal[1])
					value[key] = val
				}
				configValue.FieldByName(fieldName).Set(reflect.ValueOf(value))
			}
		}
	}
}
