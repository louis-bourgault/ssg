// Package sitepath contains the path rules shared by indexing and building.
package sitepath

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Relative returns sourcePath relative to routesDir using slash separators.
func Relative(routesDir, sourcePath string) (string, error) {
	root, err := filepath.Abs(filepath.Clean(routesDir))
	if err != nil {
		return "", fmt.Errorf("resolve routes directory: %w", err)
	}
	source, err := filepath.Abs(filepath.Clean(sourcePath))
	if err != nil {
		return "", fmt.Errorf("resolve source path: %w", err)
	}
	relative, err := filepath.Rel(root, source)
	if err != nil {
		return "", fmt.Errorf("resolve source path relative to routes: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("source path %q is outside routes directory %q", sourcePath, routesDir)
	}
	return filepath.ToSlash(relative), nil
}

// OutputPath returns the production build path for a source file.
func OutputPath(routesDir, buildDir, sourcePath, fileType string) (string, error) {
	relative, err := Relative(routesDir, sourcePath)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(fileType, "md") {
		if relative == "index.md" {
			relative = "index.html"
		} else if strings.HasSuffix(relative, "/index.md") {
			relative = strings.TrimSuffix(relative, ".md") + ".html"
		} else {
			relative = strings.TrimSuffix(relative, filepath.Ext(relative)) + "/index.html"
		}
	}
	return filepath.ToSlash(filepath.Join(buildDir, filepath.FromSlash(relative))), nil
}

// PrettyURL returns the public URL corresponding to a Markdown source.
func PrettyURL(routesDir, sourcePath string) (string, error) {
	output, err := OutputPath(routesDir, "build", sourcePath, "md")
	if err != nil {
		return "", err
	}
	path := strings.TrimPrefix(filepath.ToSlash(output), "build/")
	path = strings.TrimSuffix(path, "index.html")
	if path == "" {
		return "/", nil
	}
	return "/" + strings.TrimPrefix(path, "/"), nil
}
