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
	"strconv"

	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/config"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/controller"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/model"
	"github.com/gin-gonic/gin"
)

func init() {
	endpoints = append(endpoints, RulesEndpoint)
}

func RulesEndpoint(router gin.IRoutes, _ config.Config, control controller.Controller) {
	router.GET("/rules", func(c *gin.Context) {
		limitStr := c.Query("limit")
		var limit int
		var err error
		if len(limitStr) > 0 {
			limit, err = strconv.Atoi(limitStr)
			if err != nil {
				c.Error(errors.Join(model.ErrBadRequest, err))
				return
			}
		} else {
			limit = 50
		}

		offsetStr := c.Query("offset")
		var offset int
		if len(offsetStr) > 0 {
			offset, err = strconv.Atoi(offsetStr)
			if err != nil {
				c.Error(errors.Join(model.ErrBadRequest, err))
				return
			}
		} else {
			offset = 0
		}

		rules, code, err := control.ListRules(limit, offset)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(code), err))
			return
		}
		c.Header("Content-Type", "application/json")
		err = json.NewEncoder(c.Writer).Encode(rules)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrInternalServerError, err))
			return
		}
	})

	router.GET("/rules/:id", func(c *gin.Context) {
		id := c.Param("id")
		rule, code, err := control.GetRule(id)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(code), err))
			return
		}
		c.Header("Content-Type", "application/json")
		err = json.NewEncoder(c.Writer).Encode(rule)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrInternalServerError, err))
			return
		}
	})

	router.POST("/rules", func(c *gin.Context) {
		rule := model.Rule{}
		err := c.ShouldBindJSON(&rule)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		respRule, code, err := control.CreateRule(&rule)
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

	router.PUT("/rules/:id", func(c *gin.Context) {
		rule := model.Rule{}
		err := c.ShouldBindJSON(&rule)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		id := c.Param("id")
		if id != rule.Id {
			_ = c.Error(errors.Join(model.ErrBadRequest, errors.New("ids don't match")))
			return
		}
		code, err := control.UpdateRule(&rule)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(code), err))
			return
		}
		c.Status(http.StatusOK)
	})

	router.DELETE("/rules/:id", func(c *gin.Context) {
		id := c.Param("id")
		code, err := control.DeleteRule(id)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(code), err))
			return
		}
		c.Status(http.StatusOK)
	})
}
