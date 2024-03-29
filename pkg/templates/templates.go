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

package templates

import (
	"encoding/json"
	"errors"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/config"
	"github.com/fsnotify/fsnotify"
	"log"
	"os"
	"strings"
	"sync"
)

type Template struct {
	CommandTemplate string `json:"command_template"`
	DeleteTemplate  string `json:"delete_template"`
	Description     string `json:"description"`
	Priority        int    `json:"priority"`
	Group           string `json:"group"`
}

type TemplateStore struct {
	Templates map[string]Template
	mux       sync.Mutex
}

var singleton *TemplateStore

func New(c *config.Config) (*TemplateStore, error) {
	if singleton != nil {
		return singleton, nil
	}
	if c == nil {
		return nil, errors.New("config can only be nil if singleton has been created with config")
	}
	singleton = &TemplateStore{Templates: make(map[string]Template), mux: sync.Mutex{}}
	log.Println("Reading templates from " + c.TemplateDir)
	files, err := os.ReadDir(c.TemplateDir)
	if err != nil {
		return nil, err
	}
	singleton.mux.Lock()
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			log.Println("Ignoring template in " + file.Name() + ": is dir or does not end in .json")
			continue
		}
		tmpl, err := os.ReadFile(c.TemplateDir + "/" + file.Name())
		if err != nil {
			return nil, err
		}
		var ruleTmpl Template
		err = json.Unmarshal(tmpl, &ruleTmpl)
		if err != nil {
			return nil, err
		}
		singleton.Templates[strings.TrimSuffix(file.Name(), ".json")] = ruleTmpl
	}
	singleton.mux.Unlock()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Start listening for events.
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("fsnotify event:", event)
				if !strings.HasSuffix(event.Name, ".json") {
					log.Println("Ignoring template in " + event.Name + ": does not end in .json")
					continue
				}
				switch event.Op {
				// A new pathname was created.
				case fsnotify.Create, fsnotify.Write:
					if strings.HasPrefix(event.Name, c.TemplateDir) {
						singleton.mux.Lock()
						var ruleTmpl Template
						tmpl, err := os.ReadFile(event.Name)
						if err != nil {
							log.Println("ERROR in fsnotify watcher. Templates might not update automatically: ", err)
							continue
						}
						err = json.Unmarshal(tmpl, &ruleTmpl)
						if err != nil {
							log.Println("ERROR in fsnotify watcher. Templates might not update automatically: ", err)
							continue
						}
						singleton.Templates[strings.TrimSuffix(strings.TrimPrefix(event.Name, c.TemplateDir+"/"), ".json")] = ruleTmpl
						singleton.mux.Unlock()
					}

				// fsnotify.Rename will have the template deleted, but a fsnotify.Create will also be received
				case fsnotify.Remove, fsnotify.Rename:
					if strings.HasPrefix(event.Name, c.TemplateDir) {
						singleton.mux.Lock()
						delete(singleton.Templates, strings.TrimSuffix(strings.TrimPrefix(event.Name, c.TemplateDir+"/"), ".json"))
						singleton.mux.Unlock()
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("ERROR in fsnotify watcher. Templates might not update automatically: ", err)
				continue
			}
		}
	}()

	// Add a path.
	err = watcher.Add(c.TemplateDir)
	if err != nil {
		return nil, err
	}
	return singleton, nil
}
