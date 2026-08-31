package database

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/heptau/pgarachne/internal/config"
	_ "github.com/lib/pq"
)

var (
	dbConnections = make(map[string]*sql.DB)
	dbMutex       = &sync.RWMutex{}

	// directPools holds per-user connection pools for direct-credential auth
	// (Authorization: Basic …). Keyed by directPoolKey() so that a password
	// change always triggers fresh authentication against PostgreSQL.
	directPools   = make(map[string]*directPoolEntry)
	directPoolsMu sync.RWMutex

	// poolKeyMACKey is a process-lifetime random key used to derive
	// directPoolKey's password digest via HMAC rather than a bare hash, so
	// the digest can't be fed into an offline dictionary/rainbow-table attack
	// even if it ever ended up in a log or crash dump.
	poolKeyMACKey = randomKey()
)

func randomKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(fmt.Sprintf("failed to generate pool key MAC secret: %v", err))
	}
	return key
}

// directPoolEntry pairs a per-user pool with the last time it was handed to a
// caller, so that GetUserConnection can evict long-idle entries instead of
// rejecting new credentials once the map fills up (see its use in
// evictIdleDirectPoolLocked).
type directPoolEntry struct {
	db *sql.DB
	// lastUsed is a Unix-nanosecond timestamp, accessed via sync/atomic so
	// GetUserConnection's read-locked cache-hit path can refresh it without
	// upgrading to the write lock.
	lastUsed int64
}

func (e *directPoolEntry) touch() {
	atomic.StoreInt64(&e.lastUsed, time.Now().UnixNano())
}

// defaultMaxDirectPools caps the number of distinct (user, password, dbname)
// pools to prevent unbounded memory growth under a credential-spray attack.
// Overridable via the DIRECT_POOL_LIMIT configuration variable.
const defaultMaxDirectPools = 1_000

// maxDirectPools returns the effective direct-pool cap for cfg, falling back
// to the default when the config carries no explicit limit (e.g. a Config
// struct built directly in tests).
func maxDirectPools(cfg *config.Config) int {
	if cfg != nil && cfg.DirectPoolLimit > 0 {
		return cfg.DirectPoolLimit
	}
	return defaultMaxDirectPools
}

// GetConnection returns a pooled *sql.DB for the given database name.
// The pool is created on first use and reused thereafter. database/sql manages
// connection health internally (idle timeouts, driver-level reconnects), so
// we do not Ping on every call — that would add a round-trip to every request.
// We only Ping when the cached pool has no open connections (see
// peekHealthyPool) and when promoting a pool under the write lock, where we
// need to confirm it is truly alive before handing it out to a caller that
// found the read-path pool absent or stale.
func GetConnection(cfg *config.Config, dbName string) (*sql.DB, error) {
	if db, ok := peekHealthyPool(dbName); ok {
		return db, nil
	}

	dbMutex.Lock()
	defer dbMutex.Unlock()

	// Re-check under the write lock: another goroutine may have raced us
	// here and already created a fresh pool.
	if db, ok := dbConnections[dbName]; ok {
		// Ping once under the lock to confirm the pool is reachable. This
		// is the only place we Ping — not on the hot read path.
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := db.PingContext(pingCtx)
		pingCancel()
		if err == nil {
			return db, nil
		}
		// Cached pool is dead — drop it and create a fresh one below.
		_ = db.Close()
		delete(dbConnections, dbName)
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s dbname=%s %s",
		cfg.DBHost,
		cfg.DBPort,
		config.QuoteConninfoValue(cfg.DBUser),
		config.QuoteConninfoValue(dbName),
		cfg.DBSSLParams(),
	)
	slog.Info("Creating new connection pool", "database", dbName)

	newDB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open DB connection for %s: %w", dbName, err)
	}

	// Apply pool limits so a single PgArachne instance cannot exhaust
	// PostgreSQL's connection slots. Zero values fall back to driver
	// defaults (unlimited) — the config loader substitutes conservative
	// defaults before this point, so this is a safety net.
	if cfg.DBMaxOpenConns > 0 {
		newDB.SetMaxOpenConns(cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns > 0 {
		newDB.SetMaxIdleConns(cfg.DBMaxIdleConns)
	}
	if cfg.DBConnMaxLifetime > 0 {
		newDB.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
	}
	if cfg.DBConnMaxIdleTime > 0 {
		newDB.SetConnMaxIdleTime(cfg.DBConnMaxIdleTime)
	}

	if err = newDB.Ping(); err != nil {
		_ = newDB.Close()
		return nil, fmt.Errorf("DB ping failed for %s: %w", dbName, err)
	}

	dbConnections[dbName] = newDB
	slog.Info("Successfully connected to database", "database", dbName)
	return newDB, nil
}

// peekHealthyPool returns the cached pool for dbName under a read lock.
// To keep the hot path free of network round-trips, it Pings only when the
// pool currently has no open connections: a pool that is actively serving
// traffic is healthy by definition (database/sql manages per-connection
// health internally), while a pool with zero open connections is either
// fresh, fully idle-reaped, or dead (closed pool, restarted server) — the
// single Ping distinguishes those cases. On Ping failure the pool is
// reported absent, sending the caller to GetConnection's write-locked path,
// which drops the dead pool and creates a fresh one.
func peekHealthyPool(dbName string) (*sql.DB, bool) {
	dbMutex.RLock()
	db, ok := dbConnections[dbName]
	dbMutex.RUnlock()
	if !ok {
		return nil, false
	}
	if db.Stats().OpenConnections > 0 {
		return db, true
	}
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, false
	}
	return db, true
}

// GetUserConnection returns a pooled *sql.DB authenticated directly as username
// with the supplied password. It is used for the Basic-Auth direct-credential
// mode where the caller connects as themselves rather than having PgArachne
// perform a SET LOCAL ROLE.
//
// Pool keying: sha256(password) is folded into the map key so that a password
// change always creates a fresh pool. Wrong-password attempts never reach the
// map because PostgreSQL rejects the PingContext and the pool is discarded.
//
// Pool limits: MaxOpenConns=5 per (user, db) pair, ConnMaxLifetime=5 min.
// The short lifetime bounds the window in which connections authenticated
// before a password change remain valid.
func GetUserConnection(cfg *config.Config, dbName, username, password string) (*sql.DB, error) {
	key := directPoolKey(dbName, username, password)

	directPoolsMu.RLock()
	if entry, ok := directPools[key]; ok {
		entry.touch()
		directPoolsMu.RUnlock()
		return entry.db, nil
	}
	directPoolsMu.RUnlock()

	directPoolsMu.Lock()
	defer directPoolsMu.Unlock()

	// Re-check after acquiring the write lock.
	if entry, ok := directPools[key]; ok {
		entry.touch()
		return entry.db, nil
	}

	if limit := maxDirectPools(cfg); len(directPools) >= limit {
		// The cap exists to bound memory under a credential-spray attack, but
		// long-lived deployments legitimately accumulate one entry per past
		// (user, password) combination — every password rotation leaves its
		// old pool behind forever, since nothing else ever removes a map
		// entry. Without this eviction step, enough routine rotations
		// eventually fill the map and lock out brand-new direct-auth
		// credentials even though most cached pools have long since gone
		// idle. Only a pool with zero open connections is evicted, so an
		// entry actively serving requests is never closed out from under it.
		if !evictIdleDirectPoolLocked() {
			return nil, fmt.Errorf("direct connection pool limit reached (max %d distinct credentials)", limit)
		}
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s %s",
		cfg.DBHost,
		cfg.DBPort,
		config.QuoteConninfoValue(username),
		config.QuoteConninfoValue(password),
		config.QuoteConninfoValue(dbName),
		cfg.DBSSLParams(),
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open user connection: %w", err)
	}

	// Small pool per user — direct-auth callers should not monopolise
	// PostgreSQL's connection slots.
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	// Short lifetime limits the window of stale-credential connections.
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		// Return a generic error — callers must not leak PostgreSQL's auth
		// error details (which can include the username) in HTTP responses.
		return nil, fmt.Errorf("direct authentication failed for user %q: %w", username, err)
	}

	entry := &directPoolEntry{db: db}
	entry.touch()
	directPools[key] = entry
	slog.Info("Created direct user connection pool", "database", dbName, "user", username)
	return db, nil
}

// evictIdleDirectPoolLocked scans directPools for the least-recently-used
// entry that currently has zero open connections and evicts it, closing its
// pool and returning true. It reports false when every cached pool has at
// least one open connection, meaning none can be safely reclaimed. Callers
// must hold directPoolsMu for writing.
func evictIdleDirectPoolLocked() bool {
	var victimKey string
	var victim *directPoolEntry
	for key, entry := range directPools {
		if entry.db.Stats().OpenConnections > 0 {
			continue
		}
		if victim == nil || atomic.LoadInt64(&entry.lastUsed) < atomic.LoadInt64(&victim.lastUsed) {
			victimKey, victim = key, entry
		}
	}
	if victim == nil {
		return false
	}
	delete(directPools, victimKey)
	_ = victim.db.Close()
	slog.Info("Evicted idle direct user connection pool to make room for a new one")
	return true
}

// directPoolKey builds the map key for a direct-auth pool.
// HMAC-SHA256(password) ensures a changed password creates a new pool entry
// without storing anything password-derived that could be attacked offline.
func directPoolKey(dbName, username, password string) string {
	mac := hmac.New(sha256.New, poolKeyMACKey)
	mac.Write([]byte(password))
	return username + "@" + dbName + ":" + hex.EncodeToString(mac.Sum(nil))
}

// CloseAll closes every cached connection pool. Intended for graceful
// shutdown so PostgreSQL does not have to wait for its own keep-alive
// timeout to reap the sockets. Safe to call multiple times — subsequent
// calls are no-ops because the maps are cleared after closing.
//
// Tests that build a temporary database call this in t.Cleanup so they
// do not leave open pools that interfere with the next test.
func CloseAll() {
	// Lock ordering: dbMutex before directPoolsMu — always acquire in this
	// order to prevent deadlocks with any future cross-pool operations.
	dbMutex.Lock()
	for name, db := range dbConnections {
		if err := db.Close(); err != nil {
			slog.Warn("Failed to close DB pool", "database", name, "error", err)
		}
		delete(dbConnections, name)
	}
	dbMutex.Unlock()

	directPoolsMu.Lock()
	for key, entry := range directPools {
		if err := entry.db.Close(); err != nil {
			slog.Warn("Failed to close direct user DB pool", "key", key, "error", err)
		}
		delete(directPools, key)
	}
	directPoolsMu.Unlock()
}
