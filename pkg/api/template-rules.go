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
	"errors"
	"net/http"

	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/config"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/controller"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/model"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/templates"
	"github.com/gin-gonic/gin"
)

func init() {
	endpoints = append(endpoints, TemplateRulesEndpoint)
}

func TemplateRulesEndpoint(router gin.IRoutes, _ config.Config, control controller.Controller) {
	router.POST("/template-rules", func(c *gin.Context) {
		templateRule := model.TemplateRule{}
		err := c.ShouldBindJSON(&templateRule)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		rule, err := templateRule.Rule()
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		respRule, code, err := control.CreateRule(rule)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(code), err))
			return
		}
		c.Header("Content-Type", "application/json")
		err = json.NewEncoder(c.Writer).Encode(respRule)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrInternalServerError, err))
			return
		}
	})

	router.PUT("/template-rules/:id", func(c *gin.Context) {
		caRule := model.TemplateRule{}
		err := c.ShouldBindJSON(&caRule)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		id := c.Param("id")
		if id != caRule.Id {
			_ = c.Error(errors.Join(model.ErrBadRequest, errors.New("ids don't match")))
			return
		}
		rule, err := caRule.Rule()
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		code, err := control.UpdateRule(rule)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(code), err))
			return
		}
		c.Status(http.StatusOK)
	})

	router.GET("/templates", func(c *gin.Context) {
		ts, err := templates.New(nil)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrInternalServerError, err))
			return
		}
		c.Header("Content-Type", "application/json")
		err = json.NewEncoder(c.Writer).Encode(ts.Templates)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrInternalServerError, err))
			return
		}
	})
}
