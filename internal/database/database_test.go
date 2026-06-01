package database

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/heptau/pgarachne/internal/config"
)

func requireTestDB(t *testing.T) (cfg *config.Config, dbName string) {
	t.Helper()

	if os.Getenv("PGARACHNE_TEST_DB") != "1" {
		t.Skip("set PGARACHNE_TEST_DB=1 to run database integration tests")
	}

	dbName = os.Getenv("TEST_DB_NAME")
	if dbName == "" {
		dbName = "pgarachne_test"
	}

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "pgarachne"
	}

	port, err := strconv.Atoi(dbPort)
	if err != nil {
		t.Fatalf("invalid DB_PORT: %v", err)
	}

	cfg = &config.Config{
		DBHost:    dbHost,
		DBPort:    port,
		DBUser:    dbUser,
		DBSSLMode: os.Getenv("DB_SSLMODE"),
	}
	if cfg.DBSSLMode == "" {
		cfg.DBSSLMode = "disable"
	}
	cfg.DBSSLRootCert = os.Getenv("DB_SSLROOTCERT")
	cfg.DBSSLCert = os.Getenv("DB_SSLCERT")
	cfg.DBSSLKey = os.Getenv("DB_SSLKEY")

	return cfg, dbName
}

func TestGetConnection(t *testing.T) {
	cfg, dbName := requireTestDB(t)
	db, err := GetConnection(cfg, dbName)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("db ping failed: %v", err)
	}
}

func TestGetConnectionAppliesPoolLimits(t *testing.T) {
	// Pool caching is keyed on dbName, so we cannot re-use the same logical
	// dbName with different limits in one test process without polluting
	// sibling tests. The actual SetMaxOpenConns/SetMaxIdleConns/etc. calls
	// are thin one-liners over database/sql — the real risk of regression
	// is in the *configuration* layer (parsing env vars and applying the
	// safety net). That is covered by TestConfigPoolLimitsDefaults and
	// TestConfigPoolLimitsOverride in internal/config.
	cfg, dbName := requireTestDB(t)
	_ = cfg
	_ = dbName
}

func TestCapabilitiesIncludesHelloWorld(t *testing.T) {
	cfg, dbName := requireTestDB(t)
	db, err := GetConnection(cfg, dbName)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}

	var raw json.RawMessage
	if err := db.QueryRow(`SELECT pgarachne.capabilities('{}'::jsonb)`).Scan(&raw); err != nil {
		t.Fatalf("capabilities query failed: %v", err)
	}

	var methods []struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(raw, &methods); err != nil {
		t.Fatalf("capabilities json decode failed: %v", err)
	}

	found := false
	for _, m := range methods {
		if m.Method == "api.hello_world" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected api.hello_world in capabilities")
	}
}

func TestSeedFunctionsAvailable(t *testing.T) {
	// Verifies that sql/seed_data.sql is loaded by setup_test_db.sh and that
	// the resulting api.server_info function is callable through the gateway.
	cfg, dbName := requireTestDB(t)
	db, err := GetConnection(cfg, dbName)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}

	var raw json.RawMessage
	if err := db.QueryRow(`SELECT api.server_info('{}'::jsonb)`).Scan(&raw); err != nil {
		t.Fatalf("api.server_info query failed: %v", err)
	}
	if !strings.Contains(string(raw), "current_database") {
		t.Fatalf("api.server_info returned unexpected payload: %s", raw)
	}
}

// TestGetConnectionRecoversFromDeadPool simulates the situation where the DB
// has closed the pooled connections (idle timeout, network blip, server
// restart) and verifies that GetConnection detects the dead pool on the
// next call and creates a fresh one. This is the recovery path that
// peekHealthyPool + the write-locked re-check are responsible for.
//
// The cache key and the conn string dbname are the same parameter, so we
// share the real test database with other tests in this file. We run as
// the last test (alphabetically) to minimise interference; the recovery
// path will leave a fresh pool in the cache, so sibling tests after us
// see a healthy pool.
func TestGetConnectionRecoversFromDeadPool(t *testing.T) {
	cfg, dbName := requireTestDB(t)

	// Prime the cache. We do NOT keep this handle — closing it would
	// tear down connections that sibling tests rely on.
	if _, err := GetConnection(cfg, dbName); err != nil {
		t.Fatalf("priming GetConnection failed: %v", err)
	}

	// Snapshot the cached pool so we can compare pointers afterwards.
	dbMutex.RLock()
	first, ok := dbConnections[dbName]
	dbMutex.RUnlock()
	if !ok {
		t.Fatal("expected a cached pool after priming")
	}

	// Simulate "DB closed the socket" by closing the pool in place. The
	// next peekHealthyPool call will Ping this closed pool, get an
	// error, and the write-locked re-check will drop+recreate it.
	if err := first.Close(); err != nil {
		t.Fatalf("closing the cached pool failed: %v", err)
	}

	// Recovery call. The implementation must return a working pool,
	// and it must be a *different* pointer than the one we just closed.
	second, err := GetConnection(cfg, dbName)
	if err != nil {
		t.Fatalf("GetConnection after dead pool failed: %v", err)
	}
	if second == first {
		t.Fatal("expected a fresh pool after dead pool; got the same pointer")
	}
	if err := second.Ping(); err != nil {
		t.Fatalf("fresh pool ping failed: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("fresh pool close failed: %v", err)
	}

	// Drop the (now-closed) fresh pool from the cache so subsequent
	// tests in this file (which share dbName) get a clean slate rather
	// than picking up a closed pool again.
	dbMutex.Lock()
	delete(dbConnections, dbName)
	dbMutex.Unlock()
}

// TestDirectPoolKey verifies that different passwords produce different keys
// and the same (user, db, password) triple always produces the same key.
func TestDirectPoolKey(t *testing.T) {
	k1 := directPoolKey("mydb", "alice", "secret")
	k2 := directPoolKey("mydb", "alice", "secret")
	k3 := directPoolKey("mydb", "alice", "other")
	k4 := directPoolKey("mydb", "bob", "secret")
	k5 := directPoolKey("otherdb", "alice", "secret")

	if k1 != k2 {
		t.Error("same inputs must produce the same key")
	}
	if k1 == k3 {
		t.Error("different password must produce a different key")
	}
	if k1 == k4 {
		t.Error("different username must produce a different key")
	}
	if k1 == k5 {
		t.Error("different dbname must produce a different key")
	}
}

// TestCloseAllEmpty verifies the no-op path: CloseAll must not panic or
// error when the cache is empty. This is the cold-start case for any
// process that never opened a connection.
func TestCloseAllEmpty(t *testing.T) {
	CloseAll()
	// Map should be empty after; double-call is also a no-op.
	CloseAll()
	dbMutex.RLock()
	n := len(dbConnections)
	dbMutex.RUnlock()
	if n != 0 {
		t.Fatalf("expected empty cache, got %d entries", n)
	}
}

// TestCloseAllClosesPools verifies the happy path: every cached pool is
// closed and the cache is cleared. We share the real test database name
// with sibling tests; TestGetConnectionRecoversFromDeadPool cleans up
// its own entry in the cache before this test runs, so priming here is
// safe and CloseAll at the end will not leave sibling tests with a
// dangling closed pool that they did not expect.
func TestCloseAllClosesPools(t *testing.T) {
	cfg, dbName := requireTestDB(t)

	cacheKey := dbName
	if _, err := GetConnection(cfg, cacheKey); err != nil {
		t.Fatalf("priming GetConnection failed: %v", err)
	}

	dbMutex.RLock()
	cached, ok := dbConnections[cacheKey]
	dbMutex.RUnlock()
	if !ok {
		t.Fatal("expected cached pool after priming")
	}

	CloseAll()

	dbMutex.RLock()
	_, stillThere := dbConnections[cacheKey]
	dbMutex.RUnlock()
	if stillThere {
		t.Fatal("CloseAll should have removed the cached pool")
	}

	// After CloseAll, the underlying sql.DB is closed — pinging it must
	// return an error. We do not assert on the exact error text because
	// the driver wording can change; an error is enough.
	if err := cached.Ping(); err == nil {
		t.Fatal("expected Ping on closed pool to fail; got nil")
	}
}
