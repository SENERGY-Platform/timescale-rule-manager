/*
 * Copyright 2026 InfAI (CC SES)
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
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/SENERGY-Platform/timescale-rule-manager/pkg/log"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Above the int32 range, so the key must reach pg_advisory_lock(bigint) intact.
const lockTestKey int64 = math.MaxInt64 - 4948

const (
	lockTestUser = "username"
	lockTestPw   = "password"
	lockTestDb   = "database"
)

type lockTestServer struct {
	host string
	port int
}

func startLockTestServer(t *testing.T) lockTestServer {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	// All packages of one go test run share a reaper session, and this package's reaper would remove the controller
	// tests' compose containers ten seconds after this package ends; CleanupContainer removes this container instead.
	t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	// Random host port instead of the compose file's fixed 5432, so this package can run next to the controller tests.
	ctr, err := testcontainers.Run(ctx, "timescale/timescaledb:2.25.2-pg17",
		testcontainers.WithExposedPorts("5432/tcp"),
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_USER":     lockTestUser,
			"POSTGRES_PASSWORD": lockTestPw,
			"POSTGRES_DB":       lockTestDb,
		}),
		testcontainers.WithTmpfs(map[string]string{"/var/lib/postgresql/data": "size=100M"}),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
			wait.ForListeningPort("5432/tcp"),
		),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatal(err)
	}
	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}
	return lockTestServer{host: host, port: int(port.Num())}
}

func (s lockTestServer) dsn() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", s.host, s.port, lockTestUser, lockTestPw, lockTestDb)
}

type lockTestInstance struct {
	*impl
	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

// newInstance starts an instance with its own pool, as a second replica of the service would.
func (s lockTestServer) newInstance(t *testing.T) lockTestInstance {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	db, err := New(s.host, s.port, lockTestUser, lockTestPw, lockTestDb, "public", "rules", "10s", lockTestKey, false, ctx, wg)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})
	return lockTestInstance{impl: db.(*impl), cancel: cancel, wg: wg}
}

// lockIsFree reports whether a separate session can take the lock, and releases it again if so.
func (s lockTestServer) lockIsFree(t *testing.T) bool {
	t.Helper()
	db, err := sql.Open("postgres", s.dsn())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var got bool
	err = conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1);", lockTestKey).Scan(&got)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		var released bool
		err = conn.QueryRowContext(ctx, "SELECT pg_advisory_unlock($1);", lockTestKey).Scan(&released)
		if err != nil || !released {
			t.Fatal("releasing the check lock failed", err, released)
		}
	}
	return got
}

func lockAsync(i lockTestInstance) <-chan error {
	result := make(chan error, 1)
	go func() {
		result <- i.Lock()
	}()
	return result
}

func TestAdvisoryLock(t *testing.T) {
	log.InitForTest()
	server := startLockTestServer(t)

	t.Run("unlock releases the lock although the locking connection was idle in the pool", func(t *testing.T) {
		a := server.newInstance(t)
		if err := a.Lock(); err != nil {
			t.Fatal(err)
		}
		// Hold every idle connection of the pool, so Unlock cannot reuse one of them by chance.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		idle := a.sql.Stats().Idle
		for range idle {
			conn, err := a.sql.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
		}
		if err := a.Unlock(); err != nil {
			t.Fatal(err)
		}
		if !server.lockIsFree(t) {
			t.Fatalf("lock still held after Unlock (%d idle connections held by the test)", idle)
		}
	})

	t.Run("unlock without lock returns an error", func(t *testing.T) {
		a := server.newInstance(t)
		if err := a.Unlock(); err == nil {
			t.Fatal("expected an error")
		}
		if err := a.Lock(); err != nil {
			t.Fatal(err)
		}
		if err := a.Unlock(); err != nil {
			t.Fatal(err)
		}
		if err := a.Unlock(); err == nil {
			t.Fatal("expected an error on the second Unlock")
		}
		if !server.lockIsFree(t) {
			t.Fatal("lock still held")
		}
	})

	t.Run("lock after unlock works again", func(t *testing.T) {
		a := server.newInstance(t)
		for round := range 3 {
			if err := a.Lock(); err != nil {
				t.Fatal(round, err)
			}
			if server.lockIsFree(t) {
				t.Fatal(round, "lock not held after Lock")
			}
			if err := a.Unlock(); err != nil {
				t.Fatal(round, err)
			}
			if !server.lockIsFree(t) {
				t.Fatal(round, "lock still held after Unlock")
			}
		}
	})

	t.Run("second lock on the same instance returns an error and keeps the first", func(t *testing.T) {
		a := server.newInstance(t)
		if err := a.Lock(); err != nil {
			t.Fatal(err)
		}
		if err := a.Lock(); err == nil {
			t.Fatal("expected an error")
		}
		if server.lockIsFree(t) {
			t.Fatal("lock not held")
		}
		if err := a.Unlock(); err != nil {
			t.Fatal(err)
		}
		if !server.lockIsFree(t) {
			t.Fatal("lock still held after Unlock")
		}
	})

	t.Run("second instance waits until the first unlocks", func(t *testing.T) {
		a := server.newInstance(t)
		b := server.newInstance(t)
		if err := a.Lock(); err != nil {
			t.Fatal(err)
		}
		result := lockAsync(b)
		select {
		case err := <-result:
			t.Fatal("second instance did not wait for the lock", err)
		case <-time.After(time.Second):
		}
		if err := a.Unlock(); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("second instance did not get the lock after Unlock")
		}
		if err := b.Unlock(); err != nil {
			t.Fatal(err)
		}
		if !server.lockIsFree(t) {
			t.Fatal("lock still held after Unlock")
		}
	})

	t.Run("cancelling the context ends a waiting lock", func(t *testing.T) {
		a := server.newInstance(t)
		b := server.newInstance(t)
		if err := a.Lock(); err != nil {
			t.Fatal(err)
		}
		result := lockAsync(b)
		time.Sleep(500 * time.Millisecond)
		b.cancel()
		select {
		case err := <-result:
			if err == nil {
				t.Fatal("expected an error from a cancelled Lock")
			}
		case <-time.After(10 * time.Second):
			t.Fatal("cancelled Lock did not return")
		}
		if b.lockConn != nil {
			t.Fatal("failed Lock kept its connection")
		}
		if err := a.Unlock(); err != nil {
			t.Fatal(err)
		}
		if !server.lockIsFree(t) {
			t.Fatal("lock still held after Unlock")
		}
	})

	t.Run("unlock after shutdown still releases the lock", func(t *testing.T) {
		a := server.newInstance(t)
		if err := a.Lock(); err != nil {
			t.Fatal(err)
		}
		a.cancel()
		a.wg.Wait() // the pool is closed now
		if err := a.Unlock(); err != nil {
			t.Fatal(err)
		}
		if !server.lockIsFree(t) {
			t.Fatal("lock still held after Unlock")
		}
	})
}
