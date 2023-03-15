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

package model

type TableEditMessageMethod = string

const TableEditMessageMethodPut = "put"
const TableEditMessageMethodDelete = "delete"

type TableEditMessage struct {
	Method TableEditMessageMethod `json:"method"`
	Tables []string
}

type DoneMessageHandler = string

const DoneMessageHandlerPermissionSearch = "github.com/SENERGY-Platform/permission-search"

type PermissionSearchDoneMessage struct {
	ResourceKind string             `json:"resource_kind"`
	ResourceId   string             `json:"resource_id"`
	Handler      DoneMessageHandler `json:"handler"`
	Command      string             `json:"command"`
}
