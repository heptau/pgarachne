package database

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/yourusername/pgarachne/internal/config"
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
