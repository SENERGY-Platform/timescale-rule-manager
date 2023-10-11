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
	"encoding/json"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/config"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/controller"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/model"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"strconv"
)

func init() {
	endpoints = append(endpoints, RulesEndpoint)
}

func RulesEndpoint(router *httprouter.Router, config config.Config, control controller.Controller) {
	router.GET("/rules", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		limitStr := request.URL.Query().Get("limit")
		var limit int
		var err error
		if len(limitStr) > 0 {
			limit, err = strconv.Atoi(limitStr)
			if err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
		} else {
			limit = 50
		}

		offsetStr := request.URL.Query().Get("offset")
		var offset int
		if len(limitStr) > 0 {
			offset, err = strconv.Atoi(offsetStr)
			if err != nil {
				http.Error(writer, err.Error(), http.StatusBadRequest)
				return
			}
		} else {
			offset = 50
		}

		rules, code, err := control.ListRules(limit, offset)
		if err != nil {
			http.Error(writer, err.Error(), code)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(writer).Encode(rules)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	router.GET("/rules/:id", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		id := params.ByName("id")
		rule, code, err := control.GetRule(id)
		if err != nil {
			http.Error(writer, err.Error(), code)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(writer).Encode(rule)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	router.POST("/rules", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		rule := model.Rule{}
		err := json.NewDecoder(request.Body).Decode(&rule)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		respRule, code, err := control.CreateRule(&rule)
		if err != nil {
			http.Error(writer, err.Error(), code)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(writer).Encode(respRule)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	router.PUT("/rules/:id", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		rule := model.Rule{}
		err := json.NewDecoder(request.Body).Decode(&rule)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		id := params.ByName("id")
		if id != rule.Id {
			http.Error(writer, "Ids don't match", http.StatusBadRequest)
			return
		}
		code, err := control.UpdateRule(&rule)
		if err != nil {
			http.Error(writer, err.Error(), code)
			return
		}
	})

	router.DELETE("/rules/:id", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		id := params.ByName("id")
		code, err := control.DeleteRule(id)
		if err != nil {
			http.Error(writer, err.Error(), code)
			return
		}
	})
}
