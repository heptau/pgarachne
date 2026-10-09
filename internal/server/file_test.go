package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestValidateEntryPath(t *testing.T) {
	good := []string{"a.txt", "dir/a.txt", "mimetype", "a/b/c.d", "příloha.pdf"}
	for _, p := range good {
		if err := validateEntryPath(p); err != nil {
			t.Errorf("validateEntryPath(%q) = %v; want nil", p, err)
		}
	}
	bad := []string{
		"", "/etc/passwd", "../x", "a/../b", "a/./b", "a//b", "a/", `a\b`, `..\x`,
		"C:/x", "c:x", "a\x00b", "a\nb", "\xff\xfe", strings.Repeat("a", maxFilePathLength+1),
	}
	for _, p := range bad {
		if err := validateEntryPath(p); err == nil {
			t.Errorf("validateEntryPath(%q) = nil; want error", p)
		}
	}
}

func FuzzValidateEntryPath(f *testing.F) {
	for _, s := range []string{"a.txt", "../x", "a/../b", "/abs", `a\b`, "C:/x"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, p string) {
		if validateEntryPath(p) != nil {
			return
		}
		if strings.HasPrefix(p, "/") || strings.Contains(p, `\`) || strings.Contains(p, "\x00") {
			t.Fatalf("accepted unsafe path %q", p)
		}
		for _, seg := range strings.Split(p, "/") {
			if seg == ".." || seg == "." || seg == "" {
				t.Fatalf("accepted path %q with segment %q", p, seg)
			}
		}
	})
}

func TestSafeMIME(t *testing.T) {
	for _, ok := range []string{"text/plain", "image/webp", "text/csv; charset=utf-8", "application/vnd.api+json"} {
		if safeMIME(ok) == "" {
			t.Errorf("safeMIME(%q) rejected", ok)
		}
	}
	for _, bad := range []string{"", "text", "text/plain\r\nX-Evil: 1", "text/html; boundary=x", "a/b c", "/x", "text/plain;\n"} {
		if got := safeMIME(bad); got != "" {
			t.Errorf("safeMIME(%q) = %q; want empty", bad, got)
		}
	}
}

func TestSanitizeDownloadName(t *testing.T) {
	cases := map[string]string{
		"report.zip":             "report.zip",
		"../../etc/passwd":       "_.._etc_passwd",
		"a\"b\r\nX: y.txt":       "a_bX_ y.txt",
		"  ..hidden  ":           "hidden",
		"":                       "",
		strings.Repeat("é", 300): strings.Repeat("é", 100),
	}
	for in, want := range cases {
		if got := sanitizeDownloadName(in); got != want {
			t.Errorf("sanitizeDownloadName(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestContentDisposition(t *testing.T) {
	got := contentDisposition("příloha 1.txt")
	if !strings.Contains(got, `filename="p__loha 1.txt"`) || !strings.Contains(got, "filename*=UTF-8''p%C5%99%C3%ADloha%201.txt") {
		t.Errorf("unexpected header: %s", got)
	}
	if strings.ContainsAny(got, "\r\n") {
		t.Errorf("header contains newline: %q", got)
	}
}

func TestWriteZip(t *testing.T) {
	entries := []fileEntry{
		{path: "mimetype", content: []byte("application/epub+zip"), storeOnly: true},
		{path: "dir/a.txt", content: bytes.Repeat([]byte("a"), 1000)},
	}
	for _, level := range []int{0, 1, 9} {
		var buf bytes.Buffer
		if err := writeZip(&buf, entries, level); err != nil {
			t.Fatalf("writeZip level %d: %v", level, err)
		}
		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("level %d: not a valid zip: %v", level, err)
		}
		if len(zr.File) != 2 {
			t.Fatalf("level %d: got %d entries", level, len(zr.File))
		}
		if zr.File[0].Method != zip.Store {
			t.Errorf("level %d: store_only entry method = %d", level, zr.File[0].Method)
		}
		wantA := zip.Deflate
		if level == 0 {
			wantA = zip.Store
		}
		if zr.File[1].Method != wantA {
			t.Errorf("level %d: entry method = %d; want %d", level, zr.File[1].Method, wantA)
		}
		rc, _ := zr.File[1].Open()
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		if !bytes.Equal(data, entries[1].content) {
			t.Errorf("level %d: content mismatch", level)
		}
	}
}

// --- integration (PGARACHNE_TEST_DB=1) ---

func (e *testEnv) fileURL() string {
	return e.httpServer.URL + "/" + e.cfg.APIPrefix + "/" + e.dbName + "/file"
}

func postFile(t *testing.T, env *testEnv, token string, body map[string]any) (*http.Response, []byte) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, env.fileURL(), bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("file request: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}

func TestFileEndpoint(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()
	token := loginAndGetTokenOrSkip(t, env)

	t.Run("unauthenticated", func(t *testing.T) {
		resp, _ := postFile(t, env, "", map[string]any{"method": "api.export_documents"})
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d; want 401", resp.StatusCode)
		}
	})

	t.Run("single file direct", func(t *testing.T) {
		resp, body := postFile(t, env, token, map[string]any{"method": "api.export_documents", "params": map[string]any{"count": 1}})
		if resp.StatusCode != 200 {
			t.Fatalf("status = %d: %s", resp.StatusCode, body)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
			t.Errorf("Content-Type = %q", ct)
		}
		if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, `attachment; filename="doc-1.txt"`) {
			t.Errorf("Content-Disposition = %q", cd)
		}
		if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Error("missing nosniff")
		}
		if string(body) != "Document 1" {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("multiple files zip with custom filename", func(t *testing.T) {
		resp, body := postFile(t, env, token, map[string]any{
			"method": "api.export_documents", "params": map[string]any{"count": 3},
			"options": map[string]any{"filename": "out.zip", "compression_level": 9},
		})
		if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/zip" {
			t.Fatalf("status/type = %d %q: %s", resp.StatusCode, resp.Header.Get("Content-Type"), body)
		}
		if !strings.Contains(resp.Header.Get("Content-Disposition"), `filename="out.zip"`) {
			t.Errorf("Content-Disposition = %q", resp.Header.Get("Content-Disposition"))
		}
		zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil || len(zr.File) != 3 {
			t.Fatalf("zip: err=%v files=%d", err, len(zr.File))
		}
	})

	t.Run("force_zip single", func(t *testing.T) {
		resp, body := postFile(t, env, token, map[string]any{
			"method": "api.export_documents", "params": map[string]any{"count": 1},
			"options": map[string]any{"force_zip": true},
		})
		if resp.Header.Get("Content-Type") != "application/zip" {
			t.Fatalf("Content-Type = %q", resp.Header.Get("Content-Type"))
		}
		if zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body))); err != nil || len(zr.File) != 1 {
			t.Fatalf("zip: err=%v", err)
		}
	})

	t.Run("zero rows", func(t *testing.T) {
		resp, _ := postFile(t, env, token, map[string]any{"method": "api.export_documents", "params": map[string]any{"count": 0}})
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d; want 404", resp.StatusCode)
		}
	})

	t.Run("path traversal from function", func(t *testing.T) {
		resp, body := postFile(t, env, token, map[string]any{"method": "api.export_documents", "params": map[string]any{"bad_path": "../evil.txt"}})
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("status = %d; want 500", resp.StatusCode)
		}
		if strings.Contains(string(body), "evil") {
			t.Errorf("error leaks path: %s", body)
		}
	})

	t.Run("missing function", func(t *testing.T) {
		resp, _ := postFile(t, env, token, map[string]any{"method": "api.no_such_file_fn"})
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d; want 404", resp.StatusCode)
		}
	})

	t.Run("non file function", func(t *testing.T) {
		resp, _ := postFile(t, env, token, map[string]any{"method": "api.hello_world"})
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("status = %d; want 500", resp.StatusCode)
		}
	})

	t.Run("bad requests", func(t *testing.T) {
		for name, body := range map[string]map[string]any{
			"no method":    {},
			"bad name":     {"method": "api.x; DROP TABLE y"},
			"capabilities": {"method": "capabilities"},
			"bad level":    {"method": "api.export_documents", "options": map[string]any{"compression_level": 10}},
		} {
			if resp, _ := postFile(t, env, token, body); resp.StatusCode != http.StatusBadRequest {
				t.Errorf("%s: status = %d; want 400", name, resp.StatusCode)
			}
		}
	})

	t.Run("size cap", func(t *testing.T) {
		old := env.server.Cfg.FileMaxBytes
		env.server.Cfg.FileMaxBytes = 5
		defer func() { env.server.Cfg.FileMaxBytes = old }()
		resp, _ := postFile(t, env, token, map[string]any{"method": "api.export_documents"})
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d; want 413", resp.StatusCode)
		}
	})
}

func TestFileFunctionDiscovery(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()
	token := loginAndGetTokenOrSkip(t, env)
	prefix := "/" + env.cfg.APIPrefix + "/" + env.dbName

	// capabilities lists the file function with kind "file" and the /file endpoint.
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "capabilities", "params": map[string]any{}, "id": 1})
	req, _ := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var rpc struct {
		Result []struct {
			Method, Kind, Endpoint string
		} `json:"result"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&rpc)
	resp.Body.Close()
	found := false
	for _, m := range rpc.Result {
		switch m.Method {
		case "api.export_documents":
			found = true
			if m.Kind != "file" || m.Endpoint != prefix+"/file" {
				t.Errorf("export_documents: kind=%q endpoint=%q", m.Kind, m.Endpoint)
			}
		case "api.hello_world":
			if m.Kind != "rpc" || m.Endpoint != prefix+"/jsonrpc" {
				t.Errorf("hello_world: kind=%q endpoint=%q", m.Kind, m.Endpoint)
			}
		}
	}
	if !found {
		t.Error("api.export_documents missing from capabilities")
	}

	// OpenAPI: real /file path, no virtual /rpc path for the file method.
	status, spec, err := fetchOpenAPISpec(env, token)
	if err != nil || status != http.StatusOK {
		t.Fatalf("openapi: status=%d err=%v", status, err)
	}
	paths, _ := spec["paths"].(map[string]any)
	if _, ok := paths[prefix+"/file"]; !ok {
		t.Errorf("OpenAPI paths missing %s/file", prefix)
	}
	if _, ok := paths[prefix+"/rpc/api.export_documents"]; ok {
		t.Error("file method must not appear as a /rpc path")
	}

	// MCP tools/list must not offer file functions as tools.
	for _, name := range mcpFetchToolNames(t, env, token) {
		if name == "api.export_documents" {
			t.Error("MCP tools/list exposes a file function")
		}
	}
}
