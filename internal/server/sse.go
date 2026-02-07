package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/yourusername/pgarachne/internal/config"
	"github.com/yourusername/pgarachne/internal/database"
)

const (
	defaultSSEHeartbeat   = 20 * time.Second
	defaultSSEIdleTimeout = 90 * time.Second
	defaultSSEMaxChannels = 8
	defaultSSEMaxClients  = 1000
	defaultSSEBufferSize  = 64
	defaultSSESendTimeout = 2 * time.Second
)

type sseMessage struct {
	channel string
	data    interface{}
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

	listener := pq.NewListener(connStr, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			slog.Warn("SSE listener event", "event", ev, "error", err, "database", dbName)
		}
	})

	l := &dbListener{
		dbName:      dbName,
		listener:    listener,
		channels:    make(map[string]map[*sseClient]struct{}),
		clients:     make(map[*sseClient]struct{}),
		sendTimeout: sseSendTimeout(h.cfg),
		closed:      make(chan struct{}),
	}
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
	close(listener.closed)
	_ = listener.listener.Close()
}

func (l *dbListener) run() {
	for {
		select {
		case n := <-l.listener.Notify:
			if n == nil {
				continue
			}
			data := parseNotifyPayload(n.Extra)
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
	l.clients[client] = struct{}{}

	for _, channel := range channels {
		clients := l.channels[channel]
		if clients == nil {
			clients = make(map[*sseClient]struct{})
			l.channels[channel] = clients
			if err := l.listener.Listen(channel); err != nil {
				slog.Error("SSE LISTEN failed", "channel", channel, "database", l.dbName, "error", err)
			}
		}
		clients[client] = struct{}{}
	}
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

	msg := sseMessage{channel: channel, data: data}
	for _, client := range clients {
		select {
		case <-client.done:
			continue
		case client.ch <- msg:
		case <-time.After(l.sendTimeout):
			l.dropClient(client)
		}
	}
}

func (l *dbListener) dropClient(client *sseClient) {
	client.closeOnce.Do(func() {
		close(client.done)
		l.removeClient(client.channels, client)
	})
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

	maxChannels := s.Cfg.SSEMaxChannels
	if maxChannels <= 0 {
		maxChannels = defaultSSEMaxChannels
	}
	channels, err := parseChannels(channelsParam, maxChannels)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	writeSSEComment(c.Writer, "connected")
	flusher.Flush()

	for {
		select {
		case msg := <-client.ch:
			if err := writeSSEData(c.Writer, msg); err != nil {
				return
			}
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
		if err := json.Unmarshal([]byte(trimmed), &obj); err == nil {
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

func writeSSEData(w http.ResponseWriter, msg sseMessage) error {
	payload := map[string]interface{}{
		"channel": msg.channel,
		"data":    msg.data,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", encoded); err != nil {
		return err
	}
	return nil
}

func writeSSEComment(w http.ResponseWriter, comment string) error {
	if _, err := fmt.Fprintf(w, ": %s\n\n", comment); err != nil {
		return err
	}
	return nil
}
