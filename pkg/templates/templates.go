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

func New() (*TemplateStore, error) {
	if singleton != nil {
		return singleton, nil
	}
	singleton = &TemplateStore{Templates: make(map[string]Template), mux: sync.Mutex{}}
	templateDir := "templates"
	files, err := os.ReadDir(templateDir)
	if err != nil {
		return nil, err
	}
	singleton.mux.Lock()
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		tmpl, err := os.ReadFile(templateDir + "/" + file.Name())
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
				switch event.Op {
				// A new pathname was created.
				case fsnotify.Create, fsnotify.Write:
					if strings.HasPrefix(event.Name, templateDir) {
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
						singleton.Templates[strings.TrimSuffix(strings.TrimPrefix(event.Name, templateDir+"/"), ".json")] = ruleTmpl
						singleton.mux.Unlock()
					}

				// fsnotify.Rename will have the template deleted, but a fsnotify.Create will also be received
				case fsnotify.Remove, fsnotify.Rename:
					if strings.HasPrefix(event.Name, templateDir) {
						singleton.mux.Lock()
						delete(singleton.Templates, strings.TrimSuffix(strings.TrimPrefix(event.Name, templateDir+"/"), ".json"))
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
	err = watcher.Add(templateDir)
	if err != nil {
		return nil, err
	}
	return singleton, nil
}
