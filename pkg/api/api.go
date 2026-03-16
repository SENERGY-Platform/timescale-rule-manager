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
	"context"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"

	"net/http"

	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/api/util"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/config"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/controller"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/log"
	"github.com/julienschmidt/httprouter"
)

var endpoints = []func(router *httprouter.Router, config config.Config, control controller.Controller){}

func Start(ctx context.Context, wg *sync.WaitGroup, config config.Config, control controller.Controller) (err error) {
	log.Logger.Info("start api")
	router := Router(config, control)
	server := &http.Server{Addr: ":" + config.ApiPort, Handler: router, WriteTimeout: 10 * time.Second, ReadTimeout: 2 * time.Second, ReadHeaderTimeout: 2 * time.Second}
	wg.Add(1)
	go func() {
		log.Logger.Info("Listening on", "addr", server.Addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Logger.Error("api server error", attributes.ErrorKey, err)
			os.Exit(1)
		}
	}()
	go func() {
		<-ctx.Done()
		log.Logger.Debug("api shutdown", attributes.ErrorKey, server.Shutdown(context.Background()))
		wg.Done()
	}()
	return nil
}

func Router(config config.Config, control controller.Controller) http.Handler {
	router := httprouter.New()
	for _, e := range endpoints {
		log.Logger.Info("add endpoint", "name", runtime.FuncForPC(reflect.ValueOf(e).Pointer()).Name())
		e(router, config, control)
	}
	log.Logger.Info("add logging and cors")
	corsHandler := util.NewCors(router)
	return util.NewLogger(corsHandler)
}

func getToken(request *http.Request) string {
	return request.Header.Get("Authorization")
}

func getUserId(request *http.Request) string {
	return request.Header.Get("X-UserId")
}
