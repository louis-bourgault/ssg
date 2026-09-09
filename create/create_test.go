package create

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/louis-bourgault/ssg/builder"
	sitetemplates "github.com/louis-bourgault/ssg/templates"
)

func TestRunCreatesStarterWithoutProviderFiles(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "site")
	var output bytes.Buffer
	err := Run(Options{
		Destination: destination,
		Input:       strings.NewReader(templateChoice(t, "starter", 1)),
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
		Input:       strings.NewReader(templateChoice(t, "starter", 3)),
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

func TestBundledTemplatesBuild(t *testing.T) {
	templateNames, err := directories(sitetemplates.Files, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"docs-wiki", "news-template", "starter"} {
		if !contains(templateNames, expected) {
			t.Fatalf("bundled templates = %v, missing %q", templateNames, expected)
		}
	}

	for _, templateName := range templateNames {
		t.Run(templateName, func(t *testing.T) {
			projectDir := t.TempDir()
			if err := copyTree(sitetemplates.Files, templateName, projectDir); err != nil {
				t.Fatal(err)
			}
			if err := builder.Build(context.Background(), builder.Options{
				SourceDir: filepath.Join(projectDir, "routes"),
				OutputDir: filepath.Join(projectDir, "build"),
			}); err != nil {
				t.Fatal(err)
			}
			assertExists(t, filepath.Join(projectDir, "build", "index.html"))
			if templateName == "news-template" {
				article, err := os.ReadFile(filepath.Join(
					projectDir, "build", "stories", "example-article-one", "index.html",
				))
				if err != nil {
					t.Fatal(err)
				}
				for _, expected := range []string{
					`<h2 id="lorem-ipsum">Lorem ipsum</h2>`,
					`href="#lorem-ipsum">Lorem ipsum</a>`,
				} {
					if !strings.Contains(string(article), expected) {
						t.Fatalf("news article does not contain %q", expected)
					}
				}
			}
		})
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

func templateChoice(t *testing.T, templateName string, providerChoice int) string {
	t.Helper()
	templateNames, err := directories(sitetemplates.Files, ".")
	if err != nil {
		t.Fatal(err)
	}
	for index, name := range templateNames {
		if name == templateName {
			return fmt.Sprintf("%d\n%d\n", index+1, providerChoice)
		}
	}
	t.Fatalf("template %q is not bundled", templateName)
	return ""
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
