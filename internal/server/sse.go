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
)

type sseMessage struct {
	channel string
	data    interface{}
}

type sseClient struct {
	ch   chan sseMessage
	done chan struct{}
}

type sseHub struct {
	mu  sync.Mutex
	cfg *config.Config
	dbs map[string]*dbListener
}

type dbListener struct {
	dbName   string
	listener *pq.Listener

	mu       sync.Mutex
	channels map[string]map[*sseClient]struct{}
	closed   chan struct{}
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
		dbName:   dbName,
		listener: listener,
		channels: make(map[string]map[*sseClient]struct{}),
		closed:   make(chan struct{}),
	}
	go l.run()
	h.dbs[dbName] = l
	return l, nil
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

func (l *dbListener) addClient(channels []string, client *sseClient) {
	l.mu.Lock()
	defer l.mu.Unlock()

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
}

func (l *dbListener) removeClient(channels []string, client *sseClient) {
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
		default:
			// Drop if client is slow; avoid blocking listener goroutine.
		}
	}
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
		ch:   make(chan sseMessage, 16),
		done: make(chan struct{}),
	}
	listener.addClient(channels, client)
	defer func() {
		close(client.done)
		listener.removeClient(channels, client)
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
		if !isSafeChannelName(trimmed) {
			return nil, fmt.Errorf("invalid channel name: %s", trimmed)
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		channels = append(channels, trimmed)
	}
	if len(channels) == 0 {
		return nil, fmt.Errorf("channels query parameter is required")
	}
	if max > 0 && len(channels) > max {
		return nil, fmt.Errorf("too many channels (max %d)", max)
	}
	return channels, nil
}

func isSafeChannelName(name string) bool {
	if name == "" {
		return false
	}
	return pgIdentRe.MatchString(name) || pgQuotedIdentRe.MatchString(name)
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
