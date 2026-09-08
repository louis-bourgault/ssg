package dev

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/louis-bourgault/ssg/builder"
)

func TestInitialSiteMatchesProductionServing(t *testing.T) {
	source := siteFixture(t)
	production := filepath.Join(t.TempDir(), "build")
	if err := builder.Build(context.Background(), builder.Options{SourceDir: source, OutputDir: production}); err != nil {
		t.Fatal(err)
	}
	server, baseURL, stop := startServer(t, source)
	defer stop()
	_ = server

	productionServer := http.FileServerFS(os.DirFS(production))
	paths := []string{"/", "/nested", "/nested/", "/copied.html", "/asset.txt"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			prod := serveRecorder(productionServer, path)
			dev := getNoRedirect(t, baseURL+path)
			defer dev.Body.Close()
			devBody, _ := io.ReadAll(dev.Body)
			if dev.StatusCode != prod.status {
				t.Fatalf("status = %d, production = %d", dev.StatusCode, prod.status)
			}
			if dev.Header.Get("Location") != prod.header.Get("Location") {
				t.Fatalf("Location = %q, production = %q", dev.Header.Get("Location"), prod.header.Get("Location"))
			}
			got := strings.Replace(string(devBody), clientTag, "", 1)
			if got != prod.body {
				t.Fatalf("body differs from production:\nDEV %q\nPROD %q", got, prod.body)
			}
			if dev.Header.Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control = %q", dev.Header.Get("Cache-Control"))
			}
		})
	}

	response, err := http.Get(baseURL + "/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/css") {
		t.Fatalf("CSS Content-Type = %q", contentType)
	}
}

func TestTwoClientsReceiveOneRebuild(t *testing.T) {
	source := siteFixture(t)
	_, baseURL, stop := startServer(t, source)
	defer stop()
	first := dialReload(t, baseURL)
	defer first.Close()
	second := dialReload(t, baseURL)
	defer second.Close()
	expectType(t, first, "ready")
	expectType(t, second, "ready")

	writeDevFile(t, filepath.Join(source, "index.md"), "# Changed once")
	expectType(t, first, "reload")
	expectType(t, second, "reload")
}

func TestWatcherRebuildsForProjectWideChanges(t *testing.T) {
	source := siteFixture(t)
	_, baseURL, stop := startServer(t, source)
	defer stop()
	conn := dialReload(t, baseURL)
	defer conn.Close()
	expectType(t, conn, "ready")

	steps := []struct {
		name     string
		change   func()
		expected string
	}{
		{"template", func() {
			writeDevFile(t, filepath.Join(source, "template.html"), `<html><head></head><body class="changed">{{slot}}</body></html>`)
		}, "reload"},
		{"markdown", func() { writeDevFile(t, filepath.Join(source, "nested", "index.md"), "# Nested changed") }, "reload"},
		{"css", func() { writeDevFile(t, filepath.Join(source, "styles.css"), "body { color: green; }") }, "css"},
		{"image", func() { writeDevFile(t, filepath.Join(source, "photo.bin"), "new image bytes") }, "reload"},
		{"collection entry", func() {
			writeDevFile(t, filepath.Join(source, "posts", "one.md"), "---\ntitle: One changed\n---\nBody")
		}, "reload"},
		{"create", func() { writeDevFile(t, filepath.Join(source, "created.txt"), "created") }, "reload"},
		{"rename", func() {
			if err := os.Rename(filepath.Join(source, "created.txt"), filepath.Join(source, "renamed.txt")); err != nil {
				t.Fatal(err)
			}
		}, "reload"},
		{"delete", func() {
			if err := os.Remove(filepath.Join(source, "renamed.txt")); err != nil {
				t.Fatal(err)
			}
		}, "reload"},
		{"atomic save", func() {
			temporary := filepath.Join(source, ".index.md.tmp")
			writeDevFile(t, temporary, "# Atomic")
			if err := os.Rename(temporary, filepath.Join(source, "index.md")); err != nil {
				t.Fatal(err)
			}
		}, "reload"},
		{"new directory", func() { writeDevFile(t, filepath.Join(source, "new-dir", "page.md"), "# New directory") }, "reload"},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			step.change()
			message := expectType(t, conn, step.expected)
			if step.expected == "css" && message.Path != "/styles.css" {
				t.Fatalf("CSS path = %q", message.Path)
			}
		})
	}
}

func TestRapidEventsAreDebounced(t *testing.T) {
	source := siteFixture(t)
	server, _, stop := startServer(t, source)
	defer stop()
	server.stateMu.RLock()
	initial := server.buildCount
	server.stateMu.RUnlock()
	for i := 0; i < 10; i++ {
		writeDevFile(t, filepath.Join(source, "rapid.txt"), fmt.Sprintf("%d", i))
	}
	waitFor(t, 5*time.Second, func() bool {
		server.stateMu.RLock()
		defer server.stateMu.RUnlock()
		return server.buildCount >= initial+1
	})
	time.Sleep(3 * server.debounce)
	server.stateMu.RLock()
	count := server.buildCount
	server.stateMu.RUnlock()
	if count != initial+1 {
		t.Fatalf("rapid edits caused %d builds, want 1", count-initial)
	}
}

func TestFailedRebuildPreservesOutputAndRecovers(t *testing.T) {
	source := siteFixture(t)
	_, baseURL, stop := startServer(t, source)
	defer stop()
	conn := dialReload(t, baseURL)
	defer conn.Close()
	expectType(t, conn, "ready")
	before := getBody(t, baseURL+"/")

	writeDevFile(t, filepath.Join(source, "template.html"), `<html><body>missing slot</body></html>`)
	errorMessage := expectType(t, conn, "error")
	if !strings.Contains(errorMessage.Message, "{{slot}}") {
		t.Fatalf("error is not useful: %q", errorMessage.Message)
	}
	if after := getBody(t, baseURL+"/"); after != before {
		t.Fatal("failed rebuild changed the served output")
	}

	writeDevFile(t, filepath.Join(source, "template.html"), `<html><head></head><body>{{slot}}</body></html>`)
	expectType(t, conn, "clear-error")
	expectType(t, conn, "reload")
}

func TestInitialFailureStartsDiagnosticServer(t *testing.T) {
	source := t.TempDir()
	writeDevFile(t, filepath.Join(source, "index.md"), "# Broken")
	writeDevFile(t, filepath.Join(source, "template.html"), "<html>missing slot</html>")
	_, baseURL, stop := startServer(t, source)
	defer stop()
	response, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusInternalServerError || !strings.Contains(string(body), "must contain") {
		t.Fatalf("diagnostic response: status %d body %s", response.StatusCode, body)
	}
	conn := dialReload(t, baseURL)
	defer conn.Close()
	expectType(t, conn, "ready")
	expectType(t, conn, "error")
}

func TestGeneratedOutputCannotBeEscaped(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "routes")
	writeDevFile(t, filepath.Join(source, "index.md"), "# Home")
	writeDevFile(t, filepath.Join(root, "secret.txt"), "TOP SECRET")
	_, baseURL, stop := startServer(t, source)
	defer stop()
	response, err := http.Get(baseURL + "/%2e%2e/secret.txt")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if strings.Contains(string(body), "TOP SECRET") {
		t.Fatal("request escaped the generated output directory")
	}
}

func TestContextCancellationStopsServer(t *testing.T) {
	source := siteFixture(t)
	output := filepath.Join(t.TempDir(), "dev-output")
	server, err := NewServer(Config{SourceDir: source, OutputDir: output, Host: "127.0.0.1", Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx) }()
	waitFor(t, 5*time.Second, func() bool { return server.Addr() != "" })
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after cancellation")
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
}

func siteFixture(t *testing.T) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "routes")
	writeDevFile(t, filepath.Join(source, "template.html"), `<HTML><HEAD data-theme="x"><link rel="stylesheet" href="/styles.css"></HEAD><BODY>{{slot}}</BODY></HTML>`)
	writeDevFile(t, filepath.Join(source, "index.md"), "# Home")
	writeDevFile(t, filepath.Join(source, "nested", "index.md"), "# Nested")
	writeDevFile(t, filepath.Join(source, "copied.html"), "<html><body>Copied</body></html>")
	writeDevFile(t, filepath.Join(source, "styles.css"), "body { color: blue; }")
	writeDevFile(t, filepath.Join(source, "asset.txt"), "asset")
	writeDevFile(t, filepath.Join(source, "photo.bin"), "image bytes")
	writeDevFile(t, filepath.Join(source, "posts", "one.md"), "---\ntitle: One\n---\nBody")
	return source
}

func startServer(t *testing.T, source string) (*Server, string, func()) {
	t.Helper()
	server, err := NewServer(Config{SourceDir: source, OutputDir: filepath.Join(t.TempDir(), "dev"), Host: "127.0.0.1", Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx) }()
	waitFor(t, 5*time.Second, func() bool { return server.Addr() != "" })
	stop := func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("server stopped with error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("server failed to stop")
		}
	}
	return server, "http://" + server.Addr(), stop
}

func writeDevFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func dialReload(t *testing.T, baseURL string) *websocket.Conn {
	t.Helper()
	parsed, _ := url.Parse(baseURL)
	wsURL := "ws://" + parsed.Host + "/__ssg/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func expectType(t *testing.T, conn *websocket.Conn, expected string) ReloadMessage {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read WebSocket message: %v", err)
	}
	var message ReloadMessage
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}
	if message.Type != expected {
		t.Fatalf("message type = %q, want %q (%s)", message.Type, expected, data)
	}
	return message
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func getBody(t *testing.T, target string) string {
	t.Helper()
	response, err := http.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return string(body)
}

func getNoRedirect(t *testing.T, target string) *http.Response {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

type recordedResponse struct {
	status int
	header http.Header
	body   string
}

func serveRecorder(handler http.Handler, path string) recordedResponse {
	recorder := &recordingWriter{header: make(http.Header), status: http.StatusOK}
	handler.ServeHTTP(recorder, &http.Request{Method: http.MethodGet, URL: &url.URL{Path: path}})
	return recordedResponse{recorder.status, recorder.header, recorder.body.String()}
}

type recordingWriter struct {
	header http.Header
	status int
	body   strings.Builder
	mu     sync.Mutex
}

func (r *recordingWriter) Header() http.Header    { return r.header }
func (r *recordingWriter) WriteHeader(status int) { r.status = status }
func (r *recordingWriter) Write(data []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.Write(data)
}
