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
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SENERGY-Platform/models/go/models"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/log"
	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/model"
	_ "github.com/lib/pq"
)

type impl struct {
	ruleSchema string
	ruleTable  string
	sql        *sql.DB
	ctx        context.Context
	timeout    time.Duration
	lockKey    int64
	lockConn   *sql.Conn
	debug      bool
}

func New(postgresHost string, postgresPort int, postgresUser string, postgresPw string, postgresDb string,
	postgresRuleSchema string, postgresRuleTable string, timeout string, lockKey int64, debug bool, ctx context.Context, wg *sync.WaitGroup) (DB, error) {
	timeoutD, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, err
	}
	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", postgresHost,
		postgresPort, postgresUser, postgresPw, postgresDb)
	log.Logger.Info("Connecting to PSQL", "host", postgresHost, "port", postgresPort, "user", postgresUser, "db", postgresDb)
	// open database
	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		return nil, err
	}

	wg.Add(1)
	go func() {
		<-ctx.Done()
		_ = db.Close()
		wg.Done()
	}()

	err = db.Ping()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	i := &impl{sql: db, ctx: ctx, ruleSchema: postgresRuleSchema, ruleTable: postgresRuleTable, timeout: timeoutD, lockKey: lockKey, debug: debug}
	return i, i.migrate()
}

func (this *impl) GetTx() (tx *sql.Tx, cancel context.CancelFunc, err error) {
	ctx, cancel := context.WithTimeout(this.ctx, this.timeout)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	tx, err = this.sql.BeginTx(ctx, nil)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return tx, cancel, nil
}

func (this *impl) InsertRule(rule *model.Rule, tx *sql.Tx) (err error) {
	fields, args := getFieldsAndArgs(rule)
	columns := make([]string, len(fields))
	placeholders := make([]string, len(fields))
	for i, field := range fields {
		columns[i] = "\"" + field + "\""
		placeholders[i] = "$" + strconv.Itoa(i+1)
	}
	query := fmt.Sprintf("INSERT INTO \"%s\".\"%s\" (%s) VALUES (%s);", this.ruleSchema, this.ruleTable,
		strings.Join(columns, ", "), strings.Join(placeholders, ", "))
	_, err = tx.Exec(query, args...)
	return err
}

func (this *impl) UpdateRule(rule *model.Rule, tx *sql.Tx) (err error) {
	fields, args := getFieldsAndArgs(rule)
	assignments := make([]string, len(fields))
	for i, field := range fields {
		assignments[i] = "\"" + field + "\" = $" + strconv.Itoa(i+1)
	}
	args = append(args, rule.Id)
	query := fmt.Sprintf("UPDATE \"%s\".\"%s\" SET %s WHERE \"Id\" = $%d;", this.ruleSchema, this.ruleTable,
		strings.Join(assignments, ", "), len(args))
	res, err := tx.Exec(query, args...)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (this *impl) DeleteRule(id string, tx *sql.Tx) (err error) {
	res, err := tx.Exec(fmt.Sprintf("DELETE FROM  \"%s\".\"%s\" WHERE \"Id\" = $1;", this.ruleSchema, this.ruleTable), id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (this *impl) GetRule(id string, tx *sql.Tx) (rule *model.Rule, err error) {
	r := tx.QueryRow(fmt.Sprintf("SELECT * FROM \"%s\".\"%s\" WHERE \"Id\" = $1", this.ruleSchema, this.ruleTable), id)
	rule = &model.Rule{}
	err = scan(r, rule)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return rule, nil
}

func (this *impl) ListRules(limit, offset int) (rules []model.Rule, err error) {
	rows, err := this.sql.Query(fmt.Sprintf("SELECT * FROM \"%s\".\"%s\" LIMIT %d OFFSET %d",
		this.ruleSchema, this.ruleTable, limit, offset))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		rule := model.Rule{}
		err = scan(rows, &rule)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (this *impl) FindMatchingTables(ruleIds []string, tx *sql.Tx) (tables []string, err error) {
	query := fmt.Sprintf("SELECT DISTINCT information_schema.tables.table_name "+
		"FROM information_schema.tables, \"%s\".\"%s\" WHERE information_schema.tables.table_schema = 'public' AND information_schema.tables.table_name ~ \"%s\".\"%s\".\"TableRegEx\" AND \"%s\".\"%s\".\"Id\" = ANY($1);",
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
	)
	return this.queryStrings(query, tx, textArray(ruleIds))
}

func (this *impl) FindMatchingRules(tables []string, tx *sql.Tx) (rules []model.Rule, err error) {
	query := fmt.Sprintf("SELECT \"%s\".\"%s\".* "+
		"FROM information_schema.tables, \"%s\".\"%s\" WHERE information_schema.tables.table_schema = 'public' AND information_schema.tables.table_name ~ \"%s\".\"%s\".\"TableRegEx\" AND information_schema.tables.table_name = ANY($1);",
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
	)
	rows, err := tx.Query(query, textArray(tables))
	if err != nil {
		return nil, err
	}
	rules = []model.Rule{}
	for rows.Next() {
		rule := model.Rule{}
		err = scan(rows, &rule)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (this *impl) FindDeviceTables(deviceId string) (tables []string, err error) {
	shortDeviceId, err := models.ShortenId(deviceId)
	if err != nil {
		return nil, err
	}
	return this.queryStrings("SELECT table_name FROM information_schema.tables WHERE table_name LIKE $1;", this.sql,
		"device:"+escapeLike(shortDeviceId)+"%")
}

func (this *impl) GetColumns(table string) (columns []string, err error) {
	return this.queryStrings("SELECT column_name FROM information_schema.columns where table_name = $1 AND table_schema = 'public';", this.sql, table)
}

func (this *impl) FindMatchingRulesWithOwnerInfo(table string, userIds []string, roles []string, limitToRuleIds []string, tx *sql.Tx) (rules []model.Rule, err error) {
	query := fmt.Sprintf("SELECT DISTINCT ON (\"%s\".\"%s\".\"Group\") \"%s\".\"%s\".* "+ // only one rule per Group
		"FROM information_schema.tables, \"%s\".\"%s\" WHERE information_schema.tables.table_schema = 'public' "+ // table is in schema public
		"AND information_schema.tables.table_name ~ \"%s\".\"%s\".\"TableRegEx\" "+ // table matches rule regex
		"AND information_schema.tables.table_name = $1 "+ // table name matches
		"AND ("+ // roles or user matches
		"	\"%s\".\"%s\".\"Roles\" && $2 "+ // any roles overlap
		"	OR \"%s\".\"%s\".\"Users\" && $3"+ // any userIds overlap
		")", // ensures DISTINCT ON selects rule with the highest Priority per Group
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
		this.ruleSchema, this.ruleTable,
	)
	args := []any{table, textArray(roles), textArray(userIds)}
	if limitToRuleIds != nil {
		query += fmt.Sprintf(" AND \"%s\".\"%s\".\"Id\" = ANY($4)",
			this.ruleSchema, this.ruleTable,
		)
		args = append(args, textArray(limitToRuleIds))
	}
	query += " ORDER BY \"Group\", \"Priority\" DESC;"
	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, err
	}
	rules = []model.Rule{}
	for rows.Next() {
		rule := model.Rule{}
		err = scan(rows, &rule)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (this *impl) Exec(query string, tx *sql.Tx) (sql.Result, error) {
	t := time.Now()
	r, err := tx.Exec(query)
	if this.debug {
		log.Logger.Debug("executed query", "duration", time.Since(t), "err", err, "query", query)
	}
	return r, err
}

func (this *impl) Lock() error {
	if this.lockConn != nil {
		return errors.New("advisory lock already held")
	}
	// A session-level advisory lock belongs to the connection that took it, so Unlock must run on this same connection.
	conn, err := this.sql.Conn(this.ctx)
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(this.ctx, "SELECT pg_advisory_lock($1);", this.lockKey)
	if err != nil {
		discardConn(conn)
		return err
	}
	this.lockConn = conn
	return nil
}

func (this *impl) Unlock() error {
	conn := this.lockConn
	if conn == nil {
		return errors.New("advisory lock not held")
	}
	this.lockConn = nil
	// Not bound to this.ctx, so the lock is still released cleanly during shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), this.timeout)
	defer cancel()
	var unlocked bool
	err := conn.QueryRowContext(ctx, "SELECT pg_advisory_unlock($1);", this.lockKey).Scan(&unlocked)
	if err == nil && !unlocked {
		err = errors.New("advisory lock was not held by its connection")
	}
	if err != nil {
		discardConn(conn)
		return err
	}
	return conn.Close()
}

// discardConn closes the physical connection instead of returning it to the pool, which ends the session and with
// it any advisory lock it may still hold.
func discardConn(conn *sql.Conn) {
	_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	_ = conn.Close()
}
