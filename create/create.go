// Package create scaffolds a new SSG site from templates embedded in the CLI.
package create

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/louis-bourgault/ssg/providers"
	"github.com/louis-bourgault/ssg/templates"
)

// Options configures an interactive project creation.
type Options struct {
	Destination string
	Input       io.Reader
	Output      io.Writer
}

// Run asks for a template and deployment provider, then creates the project.
func Run(options Options) error {
	if options.Destination == "" {
		options.Destination = "."
	}
	if options.Input == nil {
		options.Input = os.Stdin
	}
	if options.Output == nil {
		options.Output = os.Stdout
	}

	templateNames, err := directories(templates.Files, ".")
	if err != nil {
		return err
	}
	providerNames, err := directories(providers.Files, ".")
	if err != nil {
		return err
	}
	providerNames = append([]string{"none"}, providerNames...)

	reader := bufio.NewReader(options.Input)
	template, err := promptChoice(reader, options.Output, "Choose a template:", templateNames)
	if err != nil {
		return err
	}
	provider, err := promptChoice(reader, options.Output, "Choose a deployment provider:", providerNames)
	if err != nil {
		return err
	}

	createdDestination, err := prepareDestination(options.Destination)
	if err != nil {
		return err
	}
	completed := false
	defer func() {
		if !completed && createdDestination {
			_ = os.RemoveAll(options.Destination)
		}
	}()

	if err := copyTree(templates.Files, template, options.Destination); err != nil {
		return fmt.Errorf("copy template %q: %w", template, err)
	}
	if provider != "none" {
		if err := copyTree(providers.Files, provider, options.Destination); err != nil {
			return fmt.Errorf("copy provider %q: %w", provider, err)
		}
	}
	completed = true

	fmt.Fprintf(options.Output, "\nCreated %s with the %s template", options.Destination, template)
	if provider != "none" {
		fmt.Fprintf(options.Output, " and %s deployment files", provider)
	}
	fmt.Fprintln(options.Output, ".")
	fmt.Fprintln(options.Output, "Run `ssg dev` from the project directory to get started.")
	return nil
}

func directories(files fs.FS, root string) ([]string, error) {
	entries, err := fs.ReadDir(files, root)
	if err != nil {
		return nil, fmt.Errorf("read embedded %s: %w", root, err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("no choices found in embedded %s", root)
	}
	return names, nil
}

func promptChoice(reader *bufio.Reader, output io.Writer, question string, choices []string) (string, error) {
	for {
		fmt.Fprintln(output, question)
		for index, choice := range choices {
			fmt.Fprintf(output, "  %d. %s\n", index+1, displayName(choice))
		}
		fmt.Fprint(output, "> ")

		answer, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("read selection: %w", err)
		}
		answer = strings.TrimSpace(answer)
		selection, conversionErr := strconv.Atoi(answer)
		if conversionErr == nil && selection >= 1 && selection <= len(choices) {
			return choices[selection-1], nil
		}
		if errors.Is(err, io.EOF) {
			return "", errors.New("no selection provided")
		}
		fmt.Fprintf(output, "Enter a number from 1 to %d.\n\n", len(choices))
	}
}

func displayName(name string) string {
	if name == "none" {
		return "None / VPS"
	}
	parts := strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(name))
	for index := range parts {
		parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
	}
	return strings.Join(parts, " ")
}

func prepareDestination(destination string) (bool, error) {
	info, err := os.Stat(destination)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(destination, 0755); err != nil {
			return false, fmt.Errorf("create destination %q: %w", destination, err)
		}
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect destination %q: %w", destination, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("destination %q is not a directory", destination)
	}
	entries, err := os.ReadDir(destination)
	if err != nil {
		return false, fmt.Errorf("read destination %q: %w", destination, err)
	}
	if len(entries) != 0 {
		return false, fmt.Errorf("destination %q is not empty", destination)
	}
	return false, nil
}

func copyTree(files fs.FS, source, destination string) error {
	return fs.WalkDir(files, source, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative := strings.TrimPrefix(sourcePath, source)
		relative = strings.TrimPrefix(relative, "/")
		if relative == sourcePath || relative == ".." || strings.HasPrefix(relative, "../") {
			return fmt.Errorf("invalid embedded path %q", sourcePath)
		}
		target := filepath.Join(destination, filepath.FromSlash(relative))
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		contents, err := fs.ReadFile(files, sourcePath)
		if err != nil {
			return err
		}
		mode := fs.FileMode(0644)
		if filepath.Ext(target) == ".sh" {
			mode = 0755
		}
		return os.WriteFile(target, contents, mode)
	})
}
