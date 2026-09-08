package create

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCreatesStarterWithoutProviderFiles(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "site")
	var output bytes.Buffer
	err := Run(Options{
		Destination: destination,
		Input:       strings.NewReader("1\n1\n"),
		Output:      &output,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertExists(t, filepath.Join(destination, "routes", "index.md"))
	assertMissing(t, filepath.Join(destination, "vercel.json"))
	if !strings.Contains(output.String(), "Created ") {
		t.Fatalf("output does not report creation: %s", output.String())
	}
}

func TestRunAddsVercelOverlayAndExecutableScript(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "site")
	err := Run(Options{
		Destination: destination,
		Input:       strings.NewReader("1\n3\n"),
		Output:      &bytes.Buffer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertExists(t, filepath.Join(destination, "vercel.json"))
	info, err := os.Stat(filepath.Join(destination, "vercel_build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Fatalf("vercel_build.sh mode %v is not executable", info.Mode())
	}
	contents, err := os.ReadFile(filepath.Join(destination, "vercel_build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"releases/latest/download",
		"${SSG_VERSION:-latest}",
		"mktemp -d",
		`trap 'rm -rf "$working_directory"'`,
	} {
		if !strings.Contains(string(contents), expected) {
			t.Fatalf("vercel_build.sh does not contain %q", expected)
		}
	}
}

func TestRunRejectsNonEmptyDestination(t *testing.T) {
	destination := t.TempDir()
	existing := filepath.Join(destination, "keep.txt")
	if err := os.WriteFile(existing, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	err := Run(Options{
		Destination: destination,
		Input:       strings.NewReader("1\n1\n"),
		Output:      &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("Run error = %v, want non-empty destination error", err)
	}
	contents, err := os.ReadFile(existing)
	if err != nil || string(contents) != "keep" {
		t.Fatalf("existing file changed: contents=%q err=%v", contents, err)
	}
}

func TestPromptChoiceRetriesInvalidInputAndHandlesEOF(t *testing.T) {
	var output bytes.Buffer
	choice, err := promptChoice(
		bufioReader("no\n9\n2\n"),
		&output,
		"Choose:",
		[]string{"one", "two"},
	)
	if err != nil || choice != "two" {
		t.Fatalf("choice = %q, err = %v", choice, err)
	}
	if strings.Count(output.String(), "Enter a number") != 2 {
		t.Fatalf("prompt did not retry twice: %s", output.String())
	}

	_, err = promptChoice(bufioReader(""), &bytes.Buffer{}, "Choose:", []string{"one"})
	if err == nil {
		t.Fatal("promptChoice unexpectedly accepted EOF")
	}
}

func bufioReader(input string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(input))
}

func assertExists(t *testing.T, name string) {
	t.Helper()
	if _, err := os.Stat(name); err != nil {
		t.Fatalf("expected %q to exist: %v", name, err)
	}
}

func assertMissing(t *testing.T, name string) {
	t.Helper()
	if _, err := os.Stat(name); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected %q to be absent, got %v", name, err)
	}
}
