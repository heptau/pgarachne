package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/heptau/pgarachne/internal/config"
	"github.com/heptau/pgarachne/internal/database"
	"github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultSSEHeartbeat   = 20 * time.Second
	defaultSSEIdleTimeout = 90 * time.Second
	defaultSSEMaxChannels = 8
	defaultSSEMaxClients  = 1000
	defaultSSEBufferSize  = 64
	defaultSSESendTimeout = 2 * time.Second
)

var (
	sseClientsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pgarachne_sse_clients",
			Help: "Number of active SSE clients per database.",
		},
		[]string{"database"},
	)
	sseChannelsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pgarachne_sse_channels",
			Help: "Number of active SSE channels per database.",
		},
		[]string{"database"},
	)
	sseDropsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pgarachne_sse_client_drops_total",
			Help: "Total SSE client drops by database and reason.",
		},
		[]string{"database", "reason"},
	)
	sseEventsForwarded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pgarachne_sse_events_forwarded_total",
			Help: "Total PostgreSQL NOTIFY events received and dispatched to SSE clients, per database.",
		},
		[]string{"database"},
	)
	sseBytesSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pgarachne_sse_bytes_sent_total",
			Help: "Total bytes written to SSE client response streams, per database. Includes event payloads, heartbeats, and connection preamble.",
		},
		[]string{"database"},
	)
	sseListenErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pgarachne_sse_listen_errors_total",
			Help: "Total PostgreSQL LISTEN/connection errors observed by the SSE hub, per database and event type. A non-zero value warrants investigation — the hub drops all clients on reconnect so users see a brief interruption.",
		},
		[]string{"database", "event"},
	)
)

func init() {
	prometheus.MustRegister(
		sseClientsGauge,
		sseChannelsGauge,
		sseDropsCounter,
		sseEventsForwarded,
		sseBytesSent,
		sseListenErrors,
	)
	sseDropsCounter.WithLabelValues("init", "slow").Add(0)
	sseDropsCounter.WithLabelValues("init", "reconnect").Add(0)
	// Initialise label combinations so the time series are exported from
	// process start. Without this, Prometheus omits counters/gauges whose
	// WithLabelValues has never been called, making "absence" and "zero"
	// indistinguishable on a fresh deployment.
	sseEventsForwarded.WithLabelValues("init").Add(0)
	sseBytesSent.WithLabelValues("init").Add(0)
	for _, ev := range []string{"connection_attempt_failed", "disconnected", "reconnected", "connected", "other"} {
		sseListenErrors.WithLabelValues("init", ev).Add(0)
	}
}

type sseMessage struct {
	channel string
	data    interface{}
	dbName  string
}

type sseClient struct {
	ch        chan sseMessage
	done      chan struct{}
	channels  []string
	closeOnce sync.Once
}

type sseHub struct {
	mu  sync.Mutex
	cfg *config.Config
	dbs map[string]*dbListener
}

type dbListener struct {
	dbName   string
	listener *pq.Listener

	mu          sync.Mutex
	channels    map[string]map[*sseClient]struct{}
	clients     map[*sseClient]struct{}
	sendTimeout time.Duration
	closed      chan struct{}
	// runDone is closed when the run() goroutine returns; Shutdown waits on
	// it to confirm the listener loop has actually exited (closed only tells
	// run() to stop, it does not confirm the stop happened).
	runDone   chan struct{}
	closeOnce sync.Once
}

func newSSEHub(cfg *config.Config) *sseHub {
	return &sseHub{
		cfg: cfg,
		dbs: make(map[string]*dbListener),
	}
}

func (h *sseHub) getDBListener(dbName string) (*dbListener, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if l, ok := h.dbs[dbName]; ok {
		return l, nil
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s dbname=%s %s",
		h.cfg.DBHost,
		h.cfg.DBPort,
		config.QuoteConninfoValue(h.cfg.DBUser),
		config.QuoteConninfoValue(dbName),
		h.cfg.DBSSLParams(),
	)

	// l is fully allocated before pq.NewListener is called so the callback
	// can reference it directly without the holder-nil-check race that would
	// occur if a goroutine-launched callback fired before holder was assigned.
	l := &dbListener{
		dbName:      dbName,
		channels:    make(map[string]map[*sseClient]struct{}),
		clients:     make(map[*sseClient]struct{}),
		sendTimeout: sseSendTimeout(h.cfg),
		closed:      make(chan struct{}),
		runDone:     make(chan struct{}),
	}

	listener := pq.NewListener(connStr, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			slog.Warn("SSE listener event", "event", ev, "error", err, "database", dbName)
			sseListenErrors.WithLabelValues(dbName, eventName(ev)).Inc()
		}
		if ev == pq.ListenerEventReconnected || ev == pq.ListenerEventDisconnected {
			slog.Info("SSE listener reconnect detected; closing clients", "database", dbName, "event", ev)
			sseListenErrors.WithLabelValues(dbName, eventName(ev)).Inc()
			l.dropAllClients()
		}
	})
	l.listener = listener
	go l.run()
	h.dbs[dbName] = l
	return l, nil
}

func (h *sseHub) maybeRemoveListener(dbName string, listener *dbListener) {
	h.mu.Lock()
	defer h.mu.Unlock()

	current, ok := h.dbs[dbName]
	if !ok || current != listener {
		return
	}

	if listener.hasChannels() {
		return
	}

	delete(h.dbs, dbName)
	// Close() instead of closing the channel directly: closeOnce guarantees
	// a later hub Shutdown cannot double-close listener.closed.
	listener.Close()
}

func (l *dbListener) run() {
	defer close(l.runDone)
	for {
		select {
		case n := <-l.listener.Notify:
			if n == nil {
				continue
			}
			data := parseNotifyPayload(n.Extra)
			sseEventsForwarded.WithLabelValues(l.dbName).Inc()
			l.broadcast(n.Channel, data)
		case <-l.closed:
			return
		}
	}
}

func (l *dbListener) hasChannels() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.channels) > 0
}

func (l *dbListener) addClient(channels []string, client *sseClient, maxClients int) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if maxClients > 0 && len(l.clients) >= maxClients {
		return fmt.Errorf("too many SSE clients (max %d)", maxClients)
	}

	// Subscribe to new channels before registering the client so we can
	// roll back cleanly if any LISTEN call fails. A client that is never
	// registered will never receive events, which is safer than registering
	// it and silently dropping notifications for unsubscribed channels.
	var subscribed []string
	for _, channel := range channels {
		if l.channels[channel] == nil {
			if err := l.listener.Listen(channel); err != nil {
				slog.Error("SSE LISTEN failed", "channel", channel, "database", l.dbName, "error", err)
				for _, c := range subscribed {
					if err2 := l.listener.Unlisten(c); err2 != nil {
						slog.Warn("SSE UNLISTEN failed during rollback", "channel", c, "database", l.dbName, "error", err2)
					}
					delete(l.channels, c)
				}
				return fmt.Errorf("failed to subscribe to channel %q: %w", channel, err)
			}
			subscribed = append(subscribed, channel)
			l.channels[channel] = make(map[*sseClient]struct{})
		}
	}

	l.clients[client] = struct{}{}
	for _, channel := range channels {
		l.channels[channel][client] = struct{}{}
	}
	l.updateMetricsLocked()
	return nil
}

func (l *dbListener) removeClient(channels []string, client *sseClient) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, channel := range channels {
		clients := l.channels[channel]
		if clients == nil {
			continue
		}
		delete(clients, client)
		if len(clients) == 0 {
			delete(l.channels, channel)
			if err := l.listener.Unlisten(channel); err != nil {
				slog.Warn("SSE UNLISTEN failed", "channel", channel, "database", l.dbName, "error", err)
			}
		}
	}

	delete(l.clients, client)
	l.updateMetricsLocked()
	return len(l.channels) == 0
}

func (l *dbListener) broadcast(channel string, data interface{}) {
	l.mu.Lock()
	clientsMap := l.channels[channel]
	clients := make([]*sseClient, 0, len(clientsMap))
	for c := range clientsMap {
		clients = append(clients, c)
	}
	l.mu.Unlock()

	msg := sseMessage{channel: channel, data: data, dbName: l.dbName}
	for _, client := range clients {
		select {
		case <-client.done:
			continue
		case client.ch <- msg:
		case <-time.After(l.sendTimeout):
			l.dropClient(client, "slow")
		}
	}
}

func (l *dbListener) dropClient(client *sseClient, reason string) {
	client.closeOnce.Do(func() {
		close(client.done)
		l.removeClient(client.channels, client)
		if reason != "" {
			sseDropsCounter.WithLabelValues(l.dbName, reason).Inc()
		}
	})
}

func (l *dbListener) dropAllClients() {
	l.mu.Lock()
	clients := make([]*sseClient, 0, len(l.clients))
	for c := range l.clients {
		clients = append(clients, c)
	}
	l.mu.Unlock()

	for _, client := range clients {
		l.dropClient(client, "reconnect")
	}
}

// Close releases the underlying pq.Listener and signals the run() goroutine
// to exit. All currently-attached clients are dropped with reason "shutdown"
// so they return from their read loop and let the HTTP server finish
// draining. Safe to call multiple times — closeOnce guarantees the
// underlying listener is closed at most once.
func (l *dbListener) Close() {
	l.closeOnce.Do(func() {
		l.dropAllClientsWithReason("shutdown")
		if err := l.listener.Close(); err != nil {
			slog.Warn("SSE listener close failed", "database", l.dbName, "error", err)
		}
		close(l.closed)
	})
}

// dropAllClientsWithReason is like dropAllClients but with a custom reason
// label so the metric and log line distinguish "reconnect" (transient
// driver-level reconnect) from "shutdown" (operator-driven termination).
func (l *dbListener) dropAllClientsWithReason(reason string) {
	l.mu.Lock()
	clients := make([]*sseClient, 0, len(l.clients))
	for c := range l.clients {
		clients = append(clients, c)
	}
	l.mu.Unlock()

	for _, client := range clients {
		l.dropClient(client, reason)
	}
}

// Shutdown closes every active dbListener and waits for their run() loops
// to return, bounded by ctx. New SSE requests issued after Shutdown begins
// will see a fresh, empty hub (or fail to acquire a listener if the
// HTTP server has already been stopped by the caller).
//
// Returns the first ctx.DeadlineExceeded error if listeners do not exit
// before the deadline, but it always attempts to close every listener
// regardless — partial shutdown is better than leaking sockets indefinitely.
func (h *sseHub) Shutdown(ctx context.Context) error {
	h.mu.Lock()
	listeners := make([]*dbListener, 0, len(h.dbs))
	for _, l := range h.dbs {
		listeners = append(listeners, l)
	}
	// Detach the map so concurrent handleSSE calls (e.g. a request that
	// races with shutdown) cannot grab a stale listener reference.
	h.dbs = make(map[string]*dbListener)
	h.mu.Unlock()

	for _, l := range listeners {
		l.Close()
	}

	// Wait for each run() goroutine to actually return (runDone), not just
	// for the stop signal (closed) — Close() closes the latter synchronously,
	// so waiting on it would always succeed immediately without confirming
	// the loops exited. The waiter itself is ctx-aware so it cannot leak if
	// a run() loop hangs past the deadline.
	for _, l := range listeners {
		select {
		case <-l.runDone:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (l *dbListener) updateMetricsLocked() {
	sseClientsGauge.WithLabelValues(l.dbName).Set(float64(len(l.clients)))
	sseChannelsGauge.WithLabelValues(l.dbName).Set(float64(len(l.channels)))
}

func (s *Server) handleSSE(c *gin.Context) {
	databaseName := c.Param("database")
	if !isSafeDatabaseName(databaseName) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid database name"})
		return
	}

	channelsParam := c.Query("channels")
	if channelsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channels query parameter is required"})
		return
	}

	// SSE authentication mirrors the JSON-RPC and MCP endpoints:
	//   Basic Auth  → direct user pool; channel subscription runs as that user.
	//   Bearer/API  → system pool; authenticateToken validates JWT or API token.
	maxChannels := s.Cfg.SSEMaxChannels
	if maxChannels <= 0 {
		maxChannels = defaultSSEMaxChannels
	}
	channels, err := parseChannels(channelsParam, maxChannels)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if username, password, ok := parseBasicAuth(c.GetHeader("Authorization")); ok {
		if len(username) == 0 || len(username) > MaxLoginLength || len(password) > MaxPasswordLength {
			recordAuthResult("direct", "malformed")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		_, err := database.GetUserConnection(s.Cfg, databaseName, username, password)
		if err != nil {
			slog.Warn("SSE: direct authentication failed", "user", username, "database", databaseName, "error", err)
			recordAuthResult("direct", "invalid")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		recordAuthResult("direct", "success")
		// For direct auth we only verify credentials; the listener runs as the
		// pgarachne service user (SSE uses pq.Listener, not per-user connections).
	} else {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			recordAuthResult("unknown", "missing_header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is missing"})
			return
		}

		db, err := database.GetConnection(s.Cfg, databaseName)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database connection failed"})
			return
		}

		_, errMsg, status := s.authenticateToken(c, db, databaseName)
		if errMsg != "" {
			c.JSON(status, gin.H{"error": errMsg})
			return
		}
	}

	listener, err := s.sseHub.getDBListener(databaseName)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SSE listener unavailable"})
		return
	}

	client := &sseClient{
		ch:       make(chan sseMessage, sseBufferSize(s.Cfg)),
		done:     make(chan struct{}),
		channels: channels,
	}
	maxClients := sseMaxClients(s.Cfg)
	if err := listener.addClient(channels, client, maxClients); err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}
	defer func() {
		client.closeOnce.Do(func() { close(client.done) })
		if listener.removeClient(channels, client) {
			s.sseHub.maybeRemoveListener(databaseName, listener)
		}
	}()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming unsupported"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()

	heartbeat := s.Cfg.SSEHeartbeat
	if heartbeat <= 0 {
		heartbeat = defaultSSEHeartbeat
	}
	idleTimeout := s.Cfg.SSEIdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = defaultSSEIdleTimeout
	}
	heartbeatTicker := time.NewTicker(heartbeat)
	defer heartbeatTicker.Stop()

	idleTimer := time.NewTimer(idleTimeout)
	defer idleTimer.Stop()

	if err := writeSSEComment(c.Writer, "connected"); err != nil {
		return
	}
	sseBytesSent.WithLabelValues(databaseName).Add(float64(len(": connected\n\n")))
	flusher.Flush()

	for {
		select {
		case msg := <-client.ch:
			n, err := writeSSEData(c.Writer, msg)
			if err != nil {
				return
			}
			sseBytesSent.WithLabelValues(msg.dbName).Add(float64(n))
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(idleTimeout)
			flusher.Flush()
		case <-heartbeatTicker.C:
			if err := writeSSEComment(c.Writer, "ping"); err != nil {
				return
			}
			sseBytesSent.WithLabelValues(databaseName).Add(float64(len(": ping\n\n")))
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(idleTimeout)
			flusher.Flush()
		case <-idleTimer.C:
			return
		case <-c.Request.Context().Done():
			return
		case <-client.done:
			return
		}
	}
}

func parseChannels(raw string, max int) ([]string, error) {
	parts := strings.Split(raw, ",")
	if len(parts) == 0 {
		return nil, fmt.Errorf("channels query parameter is required")
	}
	channels := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		normalized, err := normalizeChannelName(trimmed)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		channels = append(channels, normalized)
	}
	if len(channels) == 0 {
		return nil, fmt.Errorf("channels query parameter is required")
	}
	if max > 0 && len(channels) > max {
		return nil, fmt.Errorf("too many channels (max %d)", max)
	}
	return channels, nil
}

func normalizeChannelName(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("invalid channel name: empty")
	}
	if pgIdentRe.MatchString(name) {
		return name, nil
	}
	if pgQuotedIdentRe.MatchString(name) {
		unquoted := strings.TrimPrefix(strings.TrimSuffix(name, `"`), `"`)
		unquoted = strings.ReplaceAll(unquoted, `""`, `"`)
		if unquoted == "" {
			return "", fmt.Errorf("invalid channel name: empty")
		}
		return unquoted, nil
	}
	return "", fmt.Errorf("invalid channel name: %s", name)
}

func parseNotifyPayload(payload string) interface{} {
	trimmed := strings.TrimSpace(payload)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var obj interface{}
		if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
			slog.Warn("SSE NOTIFY payload looks like JSON but failed to parse; forwarding as string",
				"error", err)
		} else {
			return obj
		}
	}
	return payload
}

func sseMaxClients(cfg *config.Config) int {
	if cfg == nil || cfg.SSEMaxClients <= 0 {
		return defaultSSEMaxClients
	}
	return cfg.SSEMaxClients
}

func sseBufferSize(cfg *config.Config) int {
	if cfg == nil || cfg.SSEClientBuffer <= 0 {
		return defaultSSEBufferSize
	}
	return cfg.SSEClientBuffer
}

func sseSendTimeout(cfg *config.Config) time.Duration {
	if cfg == nil || cfg.SSESendTimeout <= 0 {
		return defaultSSESendTimeout
	}
	return cfg.SSESendTimeout
}

// writeSSEData writes a single SSE "data: ..." event and returns the number
// of bytes written (including the trailing "\n\n"). The byte count feeds
// pgarachne_sse_bytes_sent_total so operators can spot bandwidth anomalies.
func writeSSEData(w http.ResponseWriter, msg sseMessage) (int, error) {
	payload := map[string]interface{}{
		"channel": msg.channel,
		"data":    msg.data,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	// Pre-compute the size we are about to write so the metric reflects
	// what the client receives even if Fprintf itself fails partway.
	total := len("data: ") + len(encoded) + 2 // 2 = the trailing "\n\n"
	if _, err := fmt.Fprintf(w, "data: %s\n\n", encoded); err != nil {
		return 0, err
	}
	return total, nil
}

func writeSSEComment(w http.ResponseWriter, comment string) error {
	if _, err := fmt.Fprintf(w, ": %s\n\n", comment); err != nil {
		return err
	}
	return nil
}

// eventName turns a pq.ListenerEventType into a stable string label for
// metrics. The pq package exposes the value as a typed integer constant
// (no String() method), so we map the known cases explicitly and fall back
// to "other" for anything new.
func eventName(ev pq.ListenerEventType) string {
	switch ev {
	case pq.ListenerEventConnectionAttemptFailed:
		return "connection_attempt_failed"
	case pq.ListenerEventDisconnected:
		return "disconnected"
	case pq.ListenerEventReconnected:
		return "reconnected"
	case pq.ListenerEventConnected:
		return "connected"
	default:
		return "other"
	}
}
