package builder

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildUsesConfiguredRootsAndReplacesOutput(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "content")
	output := filepath.Join(root, "public")
	writeFile(t, filepath.Join(source, "template.html"), `<html><body>{{slot}}</body></html>`)
	writeFile(t, filepath.Join(source, "index.md"), `[Guide](./guide.md)`)
	writeFile(t, filepath.Join(source, "guide.md"), `# Guide`)
	writeFile(t, filepath.Join(source, "copied.html"), `<h1>Copied</h1>`)
	writeFile(t, filepath.Join(source, "asset.css"), `body { color: navy; }`)

	if err := Build(context.Background(), Options{SourceDir: source, OutputDir: output}); err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	assertContains(t, filepath.Join(output, "index.html"), `href="/guide/"`)
	assertContains(t, filepath.Join(output, "guide", "index.html"), `<h1 id="guide">Guide</h1>`)
	assertContains(t, filepath.Join(output, "copied.html"), `Copied`)
	assertContains(t, filepath.Join(output, "asset.css"), `navy`)

	if err := os.Remove(filepath.Join(source, "asset.css")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(source, "new.txt"), "new")
	if err := Build(context.Background(), Options{SourceDir: source, OutputDir: output}); err != nil {
		t.Fatalf("second Build failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "asset.css")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale output was not removed: %v", err)
	}
}

func TestBuildFailurePreservesLastSuccessfulOutput(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "routes")
	output := filepath.Join(root, "output")
	writeFile(t, filepath.Join(source, "index.md"), "# Working")
	if err := Build(context.Background(), Options{SourceDir: source, OutputDir: output}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(output, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(source, "template.html"), `<html>missing slot</html>`)
	if err := Build(context.Background(), Options{SourceDir: source, OutputDir: output}); err == nil {
		t.Fatal("invalid template build succeeded")
	}
	after, err := os.ReadFile(filepath.Join(output, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("failed build changed the previous output")
	}
}

func TestBuildRejectsSourceOutputCollisionAndCancellation(t *testing.T) {
	directory := t.TempDir()
	if err := Build(context.Background(), Options{SourceDir: directory, OutputDir: directory}); err == nil {
		t.Fatal("source/output collision was accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Build(ctx, Options{SourceDir: directory, OutputDir: filepath.Join(directory, "out")}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled build returned %v", err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertContains(t *testing.T, path, fragment string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), fragment) {
		t.Fatalf("%s does not contain %q:\n%s", path, fragment, data)
	}
}
