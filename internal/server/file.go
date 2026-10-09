package server

import (
	"archive/zip"
	"compress/flate"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

const (
	// maxFilePathLength bounds a single entry path (and thus the ZIP header).
	maxFilePathLength = 512
	// maxDownloadNameLength bounds the filename placed in Content-Disposition.
	maxDownloadNameLength = 200
	defaultZipLevel       = 6
)

// FileOptions are the optional knobs of a /file request.
type FileOptions struct {
	Filename         string `json:"filename"`
	ForceZip         bool   `json:"force_zip"`
	CompressionLevel *int   `json:"compression_level"`
}

// FileRequest is the body of POST /{prefix}/{database}/file.
type FileRequest struct {
	Method         string          `json:"method"`
	Params         json.RawMessage `json:"params"`
	Options        FileOptions     `json:"options"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
}

// fileEntry is one row returned by the called function.
type fileEntry struct {
	path      string
	content   []byte
	mimeType  string
	storeOnly bool
}

// mimeRe accepts "type/subtype" with an optional charset parameter and nothing
// else, so a value coming from the database can never inject header content.
var mimeRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9!#$&^_.+-]{0,126}/[A-Za-z0-9][A-Za-z0-9!#$&^_.+-]{0,126}(; ?charset=[A-Za-z0-9._-]{1,40})?$`)

// windowsReservedRe matches DOS device names (CON, NUL, COM1, …), with or
// without an extension, which Windows cannot create as ordinary files.
var windowsReservedRe = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[0-9]|lpt[0-9])(\..*)?$`)

var driveLetterRe = regexp.MustCompile(`^[A-Za-z]:`)

// validateEntryPath rejects anything that could escape the extraction
// directory on the client (zip-slip) or that is not a plain relative file path.
func validateEntryPath(p string) error {
	switch {
	case p == "":
		return errors.New("empty path")
	case len(p) > maxFilePathLength:
		return errors.New("path too long")
	case !utf8.ValidString(p):
		return errors.New("path is not valid UTF-8")
	case strings.HasPrefix(p, "/"):
		return errors.New("absolute path")
	case strings.Contains(p, `\`):
		return errors.New("backslash in path")
	case driveLetterRe.MatchString(p):
		return errors.New("drive letter in path")
	}
	for _, r := range p {
		if r < 0x20 || r == 0x7f {
			return errors.New("control character in path")
		}
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return errors.New("invalid path segment")
		}
		// Windows extractors drop trailing dots and spaces, so "a/.. /x"
		// would become "a/../x".
		if strings.HasSuffix(seg, ".") || strings.HasSuffix(seg, " ") {
			return errors.New("segment ends with a dot or space")
		}
		// ':' would select an NTFS alternate data stream; the rest are
		// reserved on Windows.
		if strings.ContainsAny(seg, `:<>"|?*`) {
			return errors.New("reserved character in path")
		}
		if windowsReservedRe.MatchString(seg) {
			return errors.New("reserved device name")
		}
	}
	return nil
}

// safeMIME returns mt if it is a well-formed media type, otherwise "".
func safeMIME(mt string) string {
	mt = strings.TrimSpace(mt)
	if mimeRe.MatchString(mt) {
		return mt
	}
	return ""
}

// sanitizeDownloadName reduces name to something safe for Content-Disposition:
// no path separators, quotes, control characters or reserved filename chars.
func sanitizeDownloadName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r < 0x20 || r == 0x7f:
		// Bidi overrides, zero-width and line/paragraph separators would let a
		// name like "invoice\u202efdp.exe" display as something else.
		case unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) || r == 0x85:
		case strings.ContainsRune(`"\/:*?<>|;`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Trim(strings.TrimSpace(b.String()), ".")
	if len(out) > maxDownloadNameLength {
		out = out[:maxDownloadNameLength]
		for !utf8.ValidString(out) {
			out = out[:len(out)-1]
		}
	}
	return out
}

// contentDisposition builds an attachment header with an ASCII fallback and an
// RFC 6266 / 5987 UTF-8 filename*.
func contentDisposition(name string) string {
	ascii := make([]rune, 0, len(name))
	for _, r := range name {
		if r < 0x20 || r > 0x7e {
			r = '_'
		}
		ascii = append(ascii, r)
	}
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		string(ascii), strings.ReplaceAll(url.QueryEscape(name), "+", "%20"))
}

func jsonError(c *gin.Context, status, code int, msg string) {
	c.JSON(status, JSONRPCResponse{Error: &JSONRPCError{Code: code, Message: msg}})
}

func isPermissionDeniedError(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "42501"
}

func (s *Server) handleFile(c *gin.Context) {
	databaseName := c.Param("database")
	if !isSafeDatabaseName(databaseName) {
		jsonError(c, http.StatusBadRequest, 0, "Invalid database name")
		return
	}

	var req FileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, 0, "Invalid JSON request")
		return
	}

	functionName := strings.TrimSpace(req.Method)
	if functionName == "" {
		recordFile("", "error")
		jsonError(c, http.StatusBadRequest, 0, "method is required")
		return
	}
	// Unlike /jsonrpc there are no special method names: only a plain
	// schema.function identifier is accepted.
	if len(functionName) > MaxMethodLength || !pgFunctionRe.MatchString(functionName) {
		recordFile("", "error")
		jsonError(c, http.StatusBadRequest, 0, "Invalid function name")
		return
	}
	if len(req.IdempotencyKey) > MaxIdempotencyKeyLength {
		recordFile(functionName, "error")
		jsonError(c, http.StatusBadRequest, 0, "idempotencyKey is too long")
		return
	}

	level := defaultZipLevel
	if req.Options.CompressionLevel != nil {
		level = *req.Options.CompressionLevel
		if level < 0 || level > 9 {
			recordFile(functionName, "error")
			jsonError(c, http.StatusBadRequest, 0, "options.compression_level must be 0-9")
			return
		}
	}

	execDB, dbRole, authErr := s.authenticateForDatabase(c, databaseName)
	if authErr != nil {
		var af *authFailure
		errors.As(authErr, &af)
		jsonError(c, af.status, 0, af.message)
		return
	}

	params := req.Params
	if len(params) == 0 || string(params) == "null" {
		params = json.RawMessage("{}")
	}

	tx, err := execDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		slog.Error("Failed to begin transaction", "error", err)
		recordFile(functionName, "error")
		jsonError(c, http.StatusServiceUnavailable, 0, "Database unavailable")
		return
	}
	defer rollbackQuietly(tx)

	if err := s.setupRequestTx(c.Request.Context(), tx, dbRole, req.IdempotencyKey); err != nil {
		switch {
		case errors.Is(err, errIdempotencyDuplicate):
			recordFile(functionName, "duplicate")
			jsonError(c, http.StatusConflict, -32000, "This request has already been processed")
		case errors.Is(err, errIdempotencyCheckFailed):
			slog.Error("Idempotency check failed", "key", req.IdempotencyKey, "function", functionName, "error", err)
			recordFile(functionName, "error")
			jsonError(c, http.StatusInternalServerError, 0, "Idempotency check failed")
		default:
			slog.Error("Failed to SET ROLE", "role", dbRole, "error", err)
			recordFile(functionName, "error")
			jsonError(c, http.StatusForbidden, -32001, "Permission denied for the specified role")
		}
		return
	}

	// functionName is validated against pgFunctionRe above (strict
	// schema.function shape); see handleFunctionCall for the SQL-injection
	// reasoning, which applies unchanged.
	rows, err := tx.QueryContext(c.Request.Context(), fmt.Sprintf("SELECT * FROM %s($1::jsonb)", functionName), params)
	if err != nil {
		s.fileQueryError(c, functionName, err)
		return
	}
	entries, status, msg := s.collectFileEntries(rows)
	if status != 0 {
		recordFile(functionName, "error")
		jsonError(c, status, 0, msg)
		return
	}

	// "No file" is reported as 404 and the transaction is rolled back (the
	// deferred rollback), so a retry is not blocked by a consumed idempotency
	// key and nothing the function did is persisted.
	if len(entries) == 0 {
		recordFile(functionName, "empty")
		jsonError(c, http.StatusNotFound, 0, "No file returned")
		return
	}

	if err := tx.Commit(); err != nil {
		slog.Error("Transaction commit failed", "error", err)
		recordFile(functionName, "error")
		jsonError(c, http.StatusInternalServerError, 0, "Transaction commit failed")
		return
	}

	recordFile(functionName, "success")
	h := c.Writer.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox")
	h.Set("Cache-Control", "no-store")

	name := sanitizeDownloadName(req.Options.Filename)

	if len(entries) == 1 && !req.Options.ForceZip {
		e := entries[0]
		if name == "" {
			name = sanitizeDownloadName(path.Base(e.path))
		}
		if name == "" {
			name = "download"
		}
		h.Set("Content-Type", entryMIME(e))
		h.Set("Content-Disposition", contentDisposition(name))
		h.Set("Content-Length", fmt.Sprint(len(e.content)))
		c.Status(http.StatusOK)
		if _, err := c.Writer.Write(e.content); err != nil {
			slog.Debug("File write aborted", "function", functionName, "error", err)
		}
		return
	}

	if name == "" {
		name = "export-" + time.Now().UTC().Format("20060102-150405") + ".zip"
	}
	h.Set("Content-Type", "application/zip")
	h.Set("Content-Disposition", contentDisposition(name))
	c.Status(http.StatusOK)
	if err := writeZip(c.Writer, entries, level); err != nil {
		slog.Debug("ZIP write aborted", "function", functionName, "error", err)
	}
}

func (s *Server) fileQueryError(c *gin.Context, functionName string, err error) {
	slog.Error("File function call failed", "function", functionName, "error", err)
	recordFile(functionName, "error")
	switch {
	case isUndefinedFunctionError(err):
		jsonError(c, http.StatusNotFound, -32601, "Function does not exist")
	case isPermissionDeniedError(err):
		jsonError(c, http.StatusForbidden, -32001, "Permission denied")
	default:
		jsonError(c, http.StatusInternalServerError, 0, "Function call failed")
	}
}

// collectFileEntries drains rows into validated entries, enforcing the row and
// size caps. On failure it returns a non-zero HTTP status and a client-safe
// message; details go to the log.
func (s *Server) collectFileEntries(rows *sql.Rows) (entries []fileEntry, status int, msg string) {
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		slog.Error("File function: reading columns failed", "error", err)
		return nil, http.StatusInternalServerError, "Function call failed"
	}
	idx := map[string]int{}
	for i, name := range cols {
		idx[name] = i
	}
	pathIdx, okPath := idx["path"]
	contentIdx, okContent := idx["content"]
	if !okPath || !okContent {
		slog.Error("File function must return columns path and content", "columns", cols)
		return nil, http.StatusInternalServerError, "Function returned an invalid file structure"
	}
	mimeIdx, hasMIME := idx["mime_type"]
	storeIdx, hasStore := idx["store_only"]

	maxEntries, maxBytes := s.Cfg.FileMaxEntries, s.Cfg.FileMaxBytes
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	if maxBytes <= 0 {
		maxBytes = 64 * 1024 * 1024
	}

	var total int64
	seen := map[string]string{}
	for rows.Next() {
		if len(entries) >= maxEntries {
			return nil, http.StatusRequestEntityTooLarge, "Too many files in response"
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			slog.Error("File function: scan failed", "error", err)
			return nil, http.StatusInternalServerError, "Function returned an invalid file structure"
		}

		var e fileEntry
		switch v := vals[pathIdx].(type) {
		case string:
			e.path = v
		case []byte:
			e.path = string(v)
		}
		if err := validateEntryPath(e.path); err != nil {
			slog.Error("File function returned an invalid path", "path", e.path, "error", err)
			return nil, http.StatusInternalServerError, "Function returned an invalid file path"
		}
		if err := claimPath(seen, e.path); err != nil {
			slog.Error("File function returned conflicting paths", "path", e.path, "error", err)
			return nil, http.StatusInternalServerError, "Function returned a duplicate file path"
		}

		switch v := vals[contentIdx].(type) {
		case []byte:
			e.content = v
		case string:
			e.content = []byte(v)
		}
		total += int64(len(e.content))
		if total > maxBytes {
			return nil, http.StatusRequestEntityTooLarge, "Response too large"
		}
		if hasMIME {
			if v, ok := vals[mimeIdx].(string); ok {
				e.mimeType = v
			}
		}
		if hasStore {
			if v, ok := vals[storeIdx].(bool); ok {
				e.storeOnly = v
			}
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		slog.Error("File function: row iteration failed", "error", err)
		if isPermissionDeniedError(err) {
			return nil, http.StatusForbidden, "Permission denied"
		}
		return nil, http.StatusInternalServerError, "Function call failed"
	}
	return entries, 0, ""
}

// claimPath registers p in seen and fails if it collides with an earlier path:
// the same name (compared case-insensitively, because macOS and Windows
// extract onto case-insensitive filesystems), or a file that is also used as a
// directory ("a" and "a/b").
func claimPath(seen map[string]string, p string) error {
	key := strings.ToLower(p)
	if _, dup := seen[key]; dup {
		return errors.New("duplicate path")
	}
	parts := strings.Split(key, "/")
	for i := 1; i < len(parts); i++ {
		if kind, ok := seen[strings.Join(parts[:i], "/")]; ok && kind == "file" {
			return errors.New("path is nested under a file")
		}
	}
	if kind := seen[key+"/"]; kind == "dir" {
		return errors.New("path is also a directory")
	}
	seen[key] = "file"
	for i := 1; i < len(parts); i++ {
		seen[strings.Join(parts[:i], "/")+"/"] = "dir"
	}
	return nil
}

// entryMIME picks the Content-Type for a single-file response: the sanitized
// database value, else a guess from the extension, else octet-stream.
func entryMIME(e fileEntry) string {
	if mt := safeMIME(e.mimeType); mt != "" {
		return mt
	}
	if mt := safeMIME(mime.TypeByExtension(path.Ext(e.path))); mt != "" {
		return mt
	}
	return "application/octet-stream"
}

// writeZip streams entries as a ZIP archive to w. level 0 stores everything
// uncompressed; entries flagged store_only are never compressed.
func writeZip(w io.Writer, entries []fileEntry, level int) error {
	zw := zip.NewWriter(w)
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, level)
	})
	now := time.Now().UTC()
	for _, e := range entries {
		method := zip.Deflate
		if level == 0 || e.storeOnly {
			method = zip.Store
		}
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: e.path, Method: method, Modified: now})
		if err != nil {
			return err
		}
		if _, err := fw.Write(e.content); err != nil {
			return err
		}
	}
	return zw.Close()
}
