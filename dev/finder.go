package dev

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func FindFile(urlpath string) (string, error) {
	toCheck := filepath.Join("routes", filepath.FromSlash(urlpath), "index.md")
	if _, err := os.Stat(toCheck); err == nil {
		return toCheck, nil
	}

	toCheck = filepath.Join("routes", filepath.FromSlash(urlpath), "index.html")
	if _, err := os.Stat(toCheck); err == nil {
		return toCheck, nil
	}
	urlPath := strings.TrimSuffix(urlpath, "/")
	if urlPath != "" {
		toCheck = filepath.Join("routes", filepath.FromSlash(urlPath)+".md")
		if _, err := os.Stat(toCheck); err == nil {
			return toCheck, nil
		}
		toCheck = filepath.Join("routes", filepath.FromSlash(urlPath)+".html")
		if _, err := os.Stat(toCheck); err == nil {
			return toCheck, nil
		}
	}
	return "", fs.ErrNotExist
}

func FindTemplateRuntime(contentLocation string) (templateLocation string) {
	log.Println("Looking for template for content at", contentLocation)
	dir := filepath.Dir(contentLocation)
	for {
		templatePath := filepath.Join(dir, "template.html")
		if _, err := os.Stat(templatePath); err == nil {
			return templatePath
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
