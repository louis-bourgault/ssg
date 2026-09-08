package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestBuildRendersSortedCollectionsHeadingsAndLiteralMarkdownDirectives(t *testing.T) {
	projectDir := t.TempDir()
	t.Chdir(projectDir)
	writeTestFile(t, filepath.Join("routes", "template.html"), `<!doctype html><title>{{meta.title}}</title>
<nav>{{#each ./posts as post sort date desc}}<a href="{{post._url}}">{{post.title}}</a>{{/each}}</nav>
<aside>{{#each meta.headings as heading}}<a href="#{{heading.id}}">{{heading.text}}</a>{{/each}}</aside>
<main>{{slot}}</main>`)
	writeTestFile(t, filepath.Join("routes", "index.md"), "---\ntitle: Home & More\n---\n# Welcome *friend*\n\nLiteral {{meta.not_evaluated}}\n")
	writeTestFile(t, filepath.Join("routes", "posts", "template.html"), `<title>{{meta.title}}</title>{{slot}}`)
	writeTestFile(t, filepath.Join("routes", "posts", "first.md"), "---\ntitle: First\ndate: 2025-01-01\n---\n# First\n")
	writeTestFile(t, filepath.Join("routes", "posts", "second.md"), "---\ntitle: Second\ndate: 2026-01-01\n---\n# Second\n")

	if err := BuildFromDirectory("routes"); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	content, err := os.ReadFile(filepath.Join("build", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	output := string(content)
	if !strings.Contains(output, `<title>Home &amp; More</title>`) {
		t.Fatalf("metadata was not escaped: %s", output)
	}
	if strings.Index(output, ">Second<") > strings.Index(output, ">First<") || strings.Index(output, ">Second<") < 0 {
		t.Fatalf("posts were not sorted descending: %s", output)
	}
	if !strings.Contains(output, `<a href="#welcome-friend">Welcome friend</a>`) {
		t.Fatalf("heading collection missing: %s", output)
	}
	if !strings.Contains(output, `Literal {{meta.not_evaluated}}`) {
		t.Fatalf("Markdown directive was evaluated: %s", output)
	}
	assertDoesNotExist(t, filepath.Join("build", "template.html"))
	assertDoesNotExist(t, filepath.Join("build", "posts", "template.html"))
}

func TestBuildUsesNearestInheritedTemplate(t *testing.T) {
	projectDir := t.TempDir()
	t.Chdir(projectDir)
	writeTestFile(t, filepath.Join("routes", "template.html"), `<div class="root">{{slot}}</div>`)
	writeTestFile(t, filepath.Join("routes", "root.md"), "Root")
	writeTestFile(t, filepath.Join("routes", "docs", "template.html"), `<div class="docs">{{slot}}</div>`)
	writeTestFile(t, filepath.Join("routes", "docs", "guide.md"), "Guide")
	writeTestFile(t, filepath.Join("routes", "docs", "deep", "page.md"), "Deep")

	if err := BuildFromDirectory("routes"); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	assertContainsFile(t, filepath.Join("build", "root", "index.html"), `class="root"`)
	assertContainsFile(t, filepath.Join("build", "docs", "guide", "index.html"), `class="docs"`)
	assertContainsFile(t, filepath.Join("build", "docs", "deep", "page", "index.html"), `class="docs"`)
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

func assertContainsFile(t *testing.T, filePath, expected string) {
	t.Helper()
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), expected) {
		t.Fatalf("%q does not contain %q: %s", filePath, expected, content)
	}
}
