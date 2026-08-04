package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/heptau/pgarachne/internal/config"
)

// newStaticTestRouter lays out a small site under a temp dir and returns a
// router serving it through the NoRoute static handler.
//
//	<root>/index.html
//	<root>/sub/index.html
//	<root>/404.html
//	<root>/escape -> <secret dir>          (symlink to a directory outside root)
//	<root>/secret.txt -> <secret dir>/…    (symlink to a file outside root)
func newStaticTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	base := t.TempDir()
	root := filepath.Join(base, "site")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{root, filepath.Join(root, "sub"), outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	files := map[string]string{
		filepath.Join(root, "index.html"):        "ROOT INDEX",
		filepath.Join(root, "sub", "index.html"): "SUB INDEX",
		filepath.Join(root, "404.html"):          "NOT FOUND PAGE",
		filepath.Join(outside, "secret.txt"):     "TOP SECRET",
	}
	for name, content := range files {
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "secret.txt")); err != nil {
		t.Fatalf("symlink file: %v", err)
	}

	s := &Server{Cfg: &config.Config{
		StaticFilesPath: root,
		APIPrefix:       "db",
		MaxRequestBytes: 1024,
	}}
	return s.buildRouter()
}

func TestStaticFileServing(t *testing.T) {
	router := newStaticTestRouter(t)

	cases := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{"root index", "/", http.StatusOK, "ROOT INDEX"},
		{"directory index", "/sub/", http.StatusOK, "SUB INDEX"},
		{"directory without slash", "/sub", http.StatusOK, "SUB INDEX"},
		{"exact file", "/404.html", http.StatusOK, "NOT FOUND PAGE"},
		{"missing file falls back to 404 page", "/nope.html", http.StatusNotFound, "NOT FOUND PAGE"},

		// Traversal: the URL path is cleaned before it reaches the handler, so
		// these resolve inside the root and hit the 404 fallback.
		{"dot dot traversal", "/../outside/secret.txt", http.StatusNotFound, "NOT FOUND PAGE"},
		{"nested dot dot traversal", "/sub/../../outside/secret.txt", http.StatusNotFound, "NOT FOUND PAGE"},

		// Symlinks pointing outside the root must not be followed — the
		// lexical containment check this replaced could not see these.
		{"symlinked file outside root", "/secret.txt", http.StatusNotFound, "NOT FOUND PAGE"},
		{"file via symlinked directory", "/escape/secret.txt", http.StatusNotFound, "NOT FOUND PAGE"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			router.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("GET %s status = %d; want %d", tc.path, w.Code, tc.wantStatus)
			}
			if body := strings.TrimSpace(w.Body.String()); body != tc.wantBody {
				t.Errorf("GET %s body = %q; want %q", tc.path, body, tc.wantBody)
			}
		})
	}
}

func TestStaticFileServingEncodedTraversal(t *testing.T) {
	router := newStaticTestRouter(t)

	// %2e%2e%2f survives gin's routing as literal ".." segments in
	// Request.URL.Path; os.Root must still refuse to leave the root.
	req := httptest.NewRequest(http.MethodGet, "/%2e%2e%2foutside%2fsecret.txt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if strings.Contains(w.Body.String(), "TOP SECRET") {
		t.Fatalf("encoded traversal escaped the static root: %q", w.Body.String())
	}
}

func TestStaticFileServingNo404Page(t *testing.T) {
	gin.SetMode(gin.TestMode)

	root := t.TempDir()
	s := &Server{Cfg: &config.Config{StaticFilesPath: root, APIPrefix: "db", MaxRequestBytes: 1024}}
	router := s.buildRouter()

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d; want %d", w.Code, http.StatusNotFound)
	}
	if got := w.Body.String(); got != "404 page not found" {
		t.Errorf("body = %q; want the built-in default", got)
	}
}
