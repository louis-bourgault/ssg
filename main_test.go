package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildReplacesStaleOutputAndSkipsLegacyIndexes(t *testing.T) {
	projectDir := t.TempDir()
	t.Chdir(projectDir)
	writeTestFile(t, filepath.Join("routes", "old.txt"), "old")
	writeTestFile(t, filepath.Join("routes", ".index.json"), `{"legacy":true}`)

	if err := BuildFromDirectory("routes"); err != nil {
		t.Fatalf("first build failed: %v", err)
	}
	if err := os.Remove(filepath.Join("routes", "old.txt")); err != nil {
		t.Fatalf("remove old source: %v", err)
	}
	writeTestFile(t, filepath.Join("routes", "new.txt"), "new")

	if err := BuildFromDirectory("routes"); err != nil {
		t.Fatalf("second build failed: %v", err)
	}

	assertFileContent(t, filepath.Join("build", "new.txt"), "new")
	assertDoesNotExist(t, filepath.Join("build", "old.txt"))
	assertDoesNotExist(t, filepath.Join("build", ".index.json"))
	assertDoesNotExist(t, ".projectindex.json")
}

func TestFailedBuildPreservesPreviousOutput(t *testing.T) {
	projectDir := t.TempDir()
	t.Chdir(projectDir)
	writeTestFile(t, filepath.Join("routes", "previous.txt"), "previous")

	if err := BuildFromDirectory("routes"); err != nil {
		t.Fatalf("initial build failed: %v", err)
	}

	writeTestFile(t, filepath.Join("routes", "index.md"), "# Broken build")
	writeTestFile(t, filepath.Join("routes", "template.html"), "<html><body>missing slot</body></html>")
	if err := BuildFromDirectory("routes"); err == nil {
		t.Fatal("build with an invalid template should fail")
	}

	assertFileContent(t, filepath.Join("build", "previous.txt"), "previous")
}

func writeTestFile(t *testing.T, filePath string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("create directory for %q: %v", filePath, err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write %q: %v", filePath, err)
	}
}

func assertFileContent(t *testing.T, filePath string, expected string) {
	t.Helper()
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read %q: %v", filePath, err)
	}
	if string(content) != expected {
		t.Fatalf("content of %q = %q, want %q", filePath, content, expected)
	}
}

func assertDoesNotExist(t *testing.T, filePath string) {
	t.Helper()
	if _, err := os.Stat(filePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%q exists or returned an unexpected error: %v", filePath, err)
	}
}
