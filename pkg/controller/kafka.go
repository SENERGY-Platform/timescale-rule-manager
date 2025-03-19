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

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/config"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/kafka"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/model"
)

func (this *impl) setupKafka(c config.Config, ctx context.Context, wg *sync.WaitGroup) (consumer *kafka.Consumer, needsSync bool, err error) {
	this.kafkaTopicPermissionUpdates = c.KafkaTopicPermissionUpdates
	this.kafkaTopicTableUpdates = c.KafkaTopicTableUpdates
	return kafka.NewConsumer(ctx, wg, c.KafkaBootstrap, []string{c.KafkaTopicTableUpdates, c.KafkaTopicPermissionUpdates}, c.KafkaGroupId, this.kafkaMessageHandler, this.kafkaErrorHandler, c.Debug)
}

type DeviceCommand struct {
	Command string `json:"command"`
	Id      string `json:"id"`
}

func (this *impl) kafkaMessageHandler(topic string, msg []byte, _ time.Time) error {
	err := this.lock()
	if err != nil {
		return err
	}
	this.logDebug("locked db for kafkaMessageHandler on topic " + topic + " with message " + string(msg))
	defer func() {
		this.unlock()
		this.logDebug("unlocked db for kafkaMessageHandler on topic " + topic + " with message " + string(msg))
	}()
	tx, cancel, err := this.db.GetTx()
	defer cancel()
	if err != nil {
		return err
	}
	switch topic {
	case this.kafkaTopicPermissionUpdates:
		var message DeviceCommand
		err := json.Unmarshal(msg, &message)
		if err != nil {
			return err
		}
		if message.Command == "DELETE" {
			return nil
		}
		tables, err := this.db.FindDeviceTables(message.Id)
		if err != nil {
			return err
		}
		for _, table := range tables {
			_, _, err = this.applyRulesForTable(table, false, nil, tx)
			if err != nil {
				return err
			}
		}
	case this.kafkaTopicTableUpdates:
		var message model.TableEditMessage
		err := json.Unmarshal(msg, &message)
		if err != nil {
			return err
		}
		if message.Method == model.TableEditMessageMethodDelete {
			return nil
		}
		for _, table := range message.Tables {
			_, _, err = this.applyRulesForTable(table, false, nil, tx)
			if err != nil {
				return err
			}
		}
	default:
		return errors.New("got kafka message on unexpected topic")
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (this *impl) kafkaErrorHandler(err error, consumer *kafka.Consumer) {
	log.Println("ERROR: Kafka : " + err.Error())
}
