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

package main

import (
	"context"
	"flag"
	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/config"
	_log "github.com/SENERGY-Platform/timescale-rule-manager/pkg/log"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	configLocation := flag.String("config", "config.json", "configuration file")
	flag.Parse()

	conf, err := config.LoadConfig(*configLocation)
	if err != nil {
		log.Fatal(err)
	}

	_log.Init(conf)

	ctx, cancel := context.WithCancel(context.Background())

	fatal := func(err error) {
		_log.Logger.Error("Fatal shutdown requested!", attributes.ErrorKey, err)
		cancel()
		go func() {
			<-time.After(25 * time.Second)
			panic("Components did not shutdown in time, failing hard!")
		}()
	}

	wg, err := pkg.Start(fatal, ctx, conf)
	if err != nil {
		_log.Logger.Error("Failed to start package", attributes.ErrorKey, err)
		log.Fatal(err)
	}

	go func() {
		shutdown := make(chan os.Signal, 1)
		signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
		sig := <-shutdown
		_log.Logger.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	wg.Wait()
}
