package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"

	_ "github.com/lib/pq"
	"github.com/yourusername/pgarachne/internal/config"
)

var (
	dbConnections = make(map[string]*sql.DB)
	dbMutex       = &sync.RWMutex{}
)

// GetConnection returns a specialized connection to a specific database (catalog).
// It maintains a pool of connections.
func GetConnection(cfg *config.Config, dbName string) (*sql.DB, error) {
	dbMutex.RLock()
	db, ok := dbConnections[dbName]
	dbMutex.RUnlock()
	if ok {
		if err := db.Ping(); err == nil {
			return db, nil
		}
	}

	dbMutex.Lock()
	defer dbMutex.Unlock()

	// Double check after lock
	db, ok = dbConnections[dbName]
	if ok {
		if err := db.Ping(); err == nil {
			return db, nil
		}
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=disable", cfg.DBHost, cfg.DBPort, cfg.DBUser, dbName)
	slog.Info("Creating new connection pool", "database", dbName)

	newDB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open DB connection for %s: %w", dbName, err)
	}

	if err = newDB.Ping(); err != nil {
		return nil, fmt.Errorf("DB ping failed for %s: %w", dbName, err)
	}

	dbConnections[dbName] = newDB
	slog.Info("Successfully connected to database", "database", dbName)
	return newDB, nil
}
