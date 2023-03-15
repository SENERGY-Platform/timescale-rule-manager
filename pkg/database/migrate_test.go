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

package database

import (
	"testing"
)

func TestQueryString(t *testing.T) {
	i := &impl{ruleTable: "rules", ruleSchema: "schema"}
	query := i.getMigrationQuery()
	if query !=
		"CREATE TABLE IF NOT EXISTS \"schema\".\"rules\" (\n"+
			"\"Id\" text primary key,\n"+
			"\"Description\" text,\n"+
			"\"Priority\" integer,\n"+
			"\"Group\" text,\n"+
			"\"TableRegEx\" text,\n"+
			"\"Users\" text[],\n"+
			"\"Roles\" text[],\n"+
			"\"CommandTemplate\" text,\n"+
			"\"DeleteTemplate\" text,\n"+
			"\"Errors\" text[]\n"+
			");" {
		t.Error("Unexpected result from getMigrationQuery(): " + query)
	}
}
