package dev

import (
	"context"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func (s *Server) watchLoop(ctx context.Context) {
	defer s.wg.Done()
	changed := make(map[string]struct{})
	var timer *time.Timer
	var timerC <-chan time.Time

	resetTimer := func() {
		if timer == nil {
			timer = time.NewTimer(s.debounce)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(s.debounce)
		}
		timerC = timer.C
	}
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 || s.ignored(event.Name) {
				continue
			}
			if event.Op&fsnotify.Create != 0 {
				_ = s.addRecursive(event.Name)
			}
			changed[filepath.Clean(event.Name)] = struct{}{}
			resetTimer()
		case <-timerC:
			s.enqueue(changed)
			changed = make(map[string]struct{})
			timerC = nil
		case _, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			// fsnotify errors (most commonly an overflow) demand a correctness
			// rebuild; the watcher remains active and can recover afterward.
			changed[s.config.SourceDir] = struct{}{}
			resetTimer()
		}
	}
}

func (s *Server) buildLoop(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.buildWake:
			changed := s.takePending()
			s.rebuild(ctx, changed, false)
		}
	}
}

func (s *Server) enqueue(paths map[string]struct{}) {
	if len(paths) == 0 {
		return
	}
	s.pendingMu.Lock()
	for path := range paths {
		s.pending[path] = struct{}{}
	}
	s.pendingMu.Unlock()
	select {
	case s.buildWake <- struct{}{}:
	default:
	}
}

func (s *Server) takePending() map[string]struct{} {
	s.pendingMu.Lock()
	paths := s.pending
	s.pending = make(map[string]struct{})
	s.pendingMu.Unlock()
	return paths
}

func (s *Server) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if path != s.config.SourceDir && s.ignored(path) {
			return filepath.SkipDir
		}
		return s.watcher.Add(path)
	})
}

func (s *Server) ignored(path string) bool {
	path, err := filepath.Abs(path)
	if err != nil {
		return true
	}
	path = filepath.Clean(path)
	if within(s.config.OutputDir, path) || within(s.productionOutputDir(), path) {
		return true
	}
	relative, err := filepath.Rel(s.config.SourceDir, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return true
	}
	for _, component := range strings.Split(filepath.ToSlash(relative), "/") {
		if component == ".git" || strings.HasPrefix(component, ".ssg-build-") || strings.Contains(component, "-staging-") || strings.Contains(component, "-previous-") {
			return true
		}
	}
	return false
}

func (s *Server) productionOutputDir() string {
	if filepath.Base(s.config.SourceDir) == "routes" {
		return filepath.Join(filepath.Dir(s.config.SourceDir), "build")
	}
	return filepath.Join(s.config.SourceDir, "build")
}

func (s *Server) cssChanges(changed map[string]struct{}) ([]string, bool) {
	if len(changed) == 0 {
		return nil, false
	}
	unique := make(map[string]struct{})
	for filePath := range changed {
		if !strings.EqualFold(filepath.Ext(filePath), ".css") {
			return nil, false
		}
		relative, err := filepath.Rel(s.config.SourceDir, filePath)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, false
		}
		unique["/"+strings.TrimPrefix(filepath.ToSlash(relative), "/")] = struct{}{}
	}
	paths := make([]string, 0, len(unique))
	for path := range unique {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, len(paths) > 0
}

func within(parent, candidate string) bool {
	relative, err := filepath.Rel(parent, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
