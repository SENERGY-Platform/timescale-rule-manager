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
	"github.com/julienschmidt/httprouter"
	"github.com/senergy-platform/timescale-rule-manager/pkg/api/util"
	"github.com/senergy-platform/timescale-rule-manager/pkg/config"
	"github.com/senergy-platform/timescale-rule-manager/pkg/controller"
	"log"
	"net/http"
	"reflect"
	"runtime"
	"sync"
	"time"
)

var endpoints = []func(router *httprouter.Router, config config.Config, control controller.Controller){}

func Start(ctx context.Context, wg *sync.WaitGroup, config config.Config, control controller.Controller) (err error) {
	log.Println("start api")
	router := Router(config, control)
	server := &http.Server{Addr: ":" + config.ApiPort, Handler: router, WriteTimeout: 30 * time.Second, ReadTimeout: 2 * time.Second, ReadHeaderTimeout: 2 * time.Second}
	wg.Add(1)
	go func() {
		log.Println("Listening on ", server.Addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Println("ERROR: api server error", err)
			log.Fatal(err)
		}
	}()
	go func() {
		<-ctx.Done()
		log.Println("DEBUG: api shutdown", server.Shutdown(context.Background()))
		wg.Done()
	}()
	return nil
}

func Router(config config.Config, control controller.Controller) http.Handler {
	router := httprouter.New()
	for _, e := range endpoints {
		log.Println("add endpoints: " + runtime.FuncForPC(reflect.ValueOf(e).Pointer()).Name())
		e(router, config, control)
	}
	log.Println("add logging and cors")
	corsHandler := util.NewCors(router)
	return util.NewLogger(corsHandler)
}

func getToken(request *http.Request) string {
	return request.Header.Get("Authorization")
}

func getUserId(request *http.Request) string {
	return request.Header.Get("X-UserId")
}
