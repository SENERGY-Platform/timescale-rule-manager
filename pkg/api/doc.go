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

package api

import (
	"net/http"
	"os"

	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/config"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/controller"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/log"
	"github.com/julienschmidt/httprouter"
)

const swaggerJSONLocation = "pkg/resources/swagger.json"

func init() {
	endpoints = append(endpoints, DocEndpoint)
}

func DocEndpoint(router *httprouter.Router, _ config.Config, _ controller.Controller) {
	json, readErr := os.ReadFile(swaggerJSONLocation)
	if readErr != nil {
		log.Logger.Error("ERROR reading swagger definition", "location", swaggerJSONLocation, attributes.ErrorKey, readErr)
	}

	router.GET("/doc", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		if readErr != nil {
			http.Error(writer, "Error reading doc file", http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, err := writer.Write(json)
		if err != nil {
			log.Logger.Error("ERROR writing doc file", attributes.ErrorKey, err)
		}
	})
}
