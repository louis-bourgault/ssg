package dev

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/louis-bourgault/ssg/builder"
)

const defaultDebounce = 100 * time.Millisecond

// Config configures a development server. Port zero asks the operating system
// for an ephemeral port; the command-line entry point supplies the default 8080.
type Config struct {
	SourceDir string
	OutputDir string
	Host      string
	Port      int
}

// Server owns the development HTTP server, project watcher, and reload clients.
type Server struct {
	config     Config
	httpServer *http.Server
	watcher    *fsnotify.Watcher
	hub        *ReloadHub
	fileServer http.Handler

	buildFunc func(context.Context, builder.Options) error
	debounce  time.Duration

	stateMu      sync.RWMutex
	lastBuildErr string
	hasOutput    bool
	buildCount   int

	pendingMu sync.Mutex
	pending   map[string]struct{}
	buildWake chan struct{}

	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	running     bool
	closed      bool
	address     string
	done        chan struct{}
	doneOnce    sync.Once
	closeOnce   sync.Once
	wg          sync.WaitGroup
}

// NewServer constructs a server and installs one recursive directory watcher.
func NewServer(config Config) (*Server, error) {
	if config.SourceDir == "" {
		config.SourceDir = "routes"
	}
	if config.OutputDir == "" {
		config.OutputDir = ".ssg-dev"
	}
	if config.Host == "" {
		config.Host = "127.0.0.1"
	}

	sourceDir, err := filepath.Abs(config.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("resolve source directory: %w", err)
	}
	outputDir, err := filepath.Abs(config.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("resolve development output directory: %w", err)
	}
	info, err := os.Stat(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("inspect source directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path %q is not a directory", sourceDir)
	}
	if sourceDir == outputDir || within(outputDir, sourceDir) {
		return nil, errors.New("development output directory must not contain the source directory")
	}
	config.SourceDir = sourceDir
	config.OutputDir = outputDir

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create source watcher: %w", err)
	}

	s := &Server{
		config:    config,
		watcher:   watcher,
		hub:       NewReloadHub(),
		buildFunc: builder.Build,
		debounce:  defaultDebounce,
		pending:   make(map[string]struct{}),
		buildWake: make(chan struct{}, 1),
		done:      make(chan struct{}),
	}
	if err := s.addRecursive(sourceDir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("watch source directory: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /__ssg/client.js", s.serveClient)
	mux.HandleFunc("GET /__ssg/ws", s.serveWebSocket)
	mux.HandleFunc("/", s.serveSite)
	s.fileServer = http.FileServerFS(os.DirFS(outputDir))
	s.httpServer = &http.Server{
		Handler:           noStore(mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	return s, nil
}

// Run builds the site, serves it, and blocks until ctx is cancelled or serving fails.
func (s *Server) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.lifecycleMu.Lock()
	if s.running || s.closed {
		s.lifecycleMu.Unlock()
		return errors.New("development server may only be run once")
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true
	s.lifecycleMu.Unlock()

	s.wg.Add(1)
	go s.watchLoop(runCtx)
	s.rebuild(runCtx, nil, true)

	s.wg.Add(1)
	go s.buildLoop(runCtx)

	listener, err := net.Listen("tcp", net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port)))
	if err != nil {
		s.finish(cancel)
		return fmt.Errorf("listen for development requests: %w", err)
	}
	s.lifecycleMu.Lock()
	s.address = listener.Addr().String()
	s.lifecycleMu.Unlock()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.httpServer.Serve(listener)
	}()

	var result error
	select {
	case <-runCtx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			result = fmt.Errorf("shut down development HTTP server: %w", err)
		}
		shutdownCancel()
		if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) && result == nil {
			result = fmt.Errorf("serve development site: %w", err)
		}
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			result = fmt.Errorf("serve development site: %w", err)
		}
	}

	s.finish(cancel)
	return result
}

// Close requests graceful shutdown and waits for Run to release its resources.
func (s *Server) Close() error {
	s.lifecycleMu.Lock()
	cancel := s.cancel
	running := s.running
	alreadyClosed := s.closed
	s.lifecycleMu.Unlock()
	if alreadyClosed {
		return nil
	}
	if cancel != nil {
		cancel()
	}
	if running {
		<-s.done
		return nil
	}
	s.finish(func() {})
	return nil
}

// Addr returns the bound address after Run has started listening.
func (s *Server) Addr() string {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	return s.address
}

func (s *Server) finish(cancel context.CancelFunc) {
	s.closeOnce.Do(func() {
		cancel()
		_ = s.watcher.Close()
		s.hub.Close()
		s.wg.Wait()
		s.lifecycleMu.Lock()
		s.closed = true
		s.lifecycleMu.Unlock()
		s.doneOnce.Do(func() { close(s.done) })
	})
}

func (s *Server) rebuild(ctx context.Context, changed map[string]struct{}, initial bool) {
	err := s.buildFunc(ctx, builder.Options{SourceDir: s.config.SourceDir, OutputDir: s.config.OutputDir})
	if ctx.Err() != nil {
		return
	}

	s.stateMu.Lock()
	hadError := s.lastBuildErr != ""
	s.buildCount++
	if err != nil {
		s.lastBuildErr = err.Error()
	} else {
		s.lastBuildErr = ""
		s.hasOutput = true
	}
	s.stateMu.Unlock()

	if initial {
		return
	}
	if err != nil {
		s.hub.Broadcast(ReloadMessage{Type: "error", Message: err.Error()})
		return
	}
	if hadError {
		s.hub.Broadcast(ReloadMessage{Type: "clear-error"})
	}
	cssPaths, cssOnly := s.cssChanges(changed)
	if cssOnly {
		for _, path := range cssPaths {
			s.hub.Broadcast(ReloadMessage{Type: "css", Path: path})
		}
		return
	}
	s.hub.Broadcast(ReloadMessage{Type: "reload"})
}

func (s *Server) serveClient(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = io.WriteString(w, liveReloadClient)
}

func (s *Server) serveWebSocket(w http.ResponseWriter, r *http.Request) {
	s.stateMu.RLock()
	buildErr := s.lastBuildErr
	s.stateMu.RUnlock()
	s.hub.ServeHTTP(w, r, buildErr)
}

func (s *Server) serveSite(w http.ResponseWriter, r *http.Request) {
	s.stateMu.RLock()
	hasOutput := s.hasOutput
	buildErr := s.lastBuildErr
	s.stateMu.RUnlock()
	if !hasOutput {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>SSG build failed</title><script src="/__ssg/client.js" defer></script></head><body><h1>SSG build failed</h1><pre style="white-space:pre-wrap">%s</pre></body></html>`, html.EscapeString(buildErr))
		return
	}

	buffer := newBufferedResponse()
	s.fileServer.ServeHTTP(buffer, r)
	body := buffer.body.String()
	if buffer.status == http.StatusOK && strings.HasPrefix(strings.ToLower(buffer.header.Get("Content-Type")), "text/html") {
		body = injectClientScript(body)
		buffer.header.Del("Content-Length")
	}
	copyHeader(w.Header(), buffer.header)
	w.WriteHeader(buffer.status)
	_, _ = io.WriteString(w, body)
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

type bufferedResponse struct {
	header http.Header
	status int
	body   strings.Builder
}

func newBufferedResponse() *bufferedResponse {
	return &bufferedResponse{header: make(http.Header), status: http.StatusOK}
}

func (w *bufferedResponse) Header() http.Header { return w.header }

func (w *bufferedResponse) WriteHeader(status int) {
	if w.status == http.StatusOK {
		w.status = status
	}
}

func (w *bufferedResponse) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func copyHeader(destination, source http.Header) {
	for key, values := range source {
		for _, value := range values {
			destination.Add(key, value)
		}
	}
}

// RunDevServer runs the command-line development server with safe defaults.
func RunDevServer() error {
	server, err := NewServer(Config{SourceDir: "routes", OutputDir: ".ssg-dev", Host: "127.0.0.1", Port: 8080})
	if err != nil {
		return err
	}
	defer server.Close()
	return server.Run(context.Background())
}
