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
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/templates"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func init() {
	endpoints = append(endpoints, TemplateRulesEndpoint)
}

func TemplateRulesEndpoint(router *httprouter.Router, _ config.Config, control controller.Controller) {
	router.POST("/template-rules", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		templateRule := model.TemplateRule{}
		err := json.NewDecoder(request.Body).Decode(&templateRule)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		rule, err := templateRule.Rule()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		respRule, code, err := control.CreateRule(rule)
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

	router.PUT("/template-rules/:id", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		caRule := model.TemplateRule{}
		err := json.NewDecoder(request.Body).Decode(&caRule)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		id := params.ByName("id")
		if id != caRule.Id {
			http.Error(writer, "Ids don't match", http.StatusBadRequest)
			return
		}
		rule, err := caRule.Rule()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		code, err := control.UpdateRule(rule)
		if err != nil {
			http.Error(writer, err.Error(), code)
			return
		}
	})

	router.GET("/templates", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		ts, err := templates.New(nil)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(writer).Encode(ts.Templates)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
