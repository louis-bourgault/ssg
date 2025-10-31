package dev

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/louis-bourgault/ssg/renderer"
)

//this is a dev server implementation

func RunDevServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Serving URL path", r.URL.Path)

		// if it doesnt finish with the slash, its just asking for a certain file so we just serve that
		if !strings.HasSuffix(r.URL.Path, "/") {
			staticPath := filepath.Join("routes", filepath.FromSlash(r.URL.Path))
			content, err := os.ReadFile(staticPath)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "404 Not Found: The requested resource '%s' could not be found.", r.URL.Path)
				return
			}
			w.Header().Set("Content-Type", getContentType(r.URL.Path))
			w.Write(content)
			return
		}

		//the possibilities are
		// /routes{path}index.md (or .html)
		// 2. the {name}.md file in the parent directory (where name is the last segment of the path)

		var file []byte
		var path string
		var err error

		// First, try index.md in the current directory
		toCheck := filepath.Join("routes", filepath.FromSlash(r.URL.Path), "index.md")
		file, err = os.ReadFile(toCheck)
		if err == nil {
			path = toCheck
		} else if os.IsNotExist(err) {
			toCheck = filepath.Join("routes", filepath.FromSlash(r.URL.Path), "index.html")
			file, err = os.ReadFile(toCheck)
			if err == nil {
				path = toCheck
				w.Write(file) //just serve html file without processing
				return
			} else if os.IsNotExist(err) {
				urlPath := strings.TrimSuffix(r.URL.Path, "/")
				if urlPath != "" {
					toCheck = filepath.Join("routes", filepath.FromSlash(urlPath)+".md")
					file, err = os.ReadFile(toCheck)
					if err == nil {
						path = toCheck
					} else if os.IsNotExist(err) {
						toCheck = filepath.Join("routes", filepath.FromSlash(urlPath)+".html")
						file, err = os.ReadFile(toCheck)
						if err == nil {
							path = toCheck
						}
						w.Write(file) //we don't process html files, we just serve them
						return
					}
				}
			}
		}

		if path == "" || err != nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "404 Not Found: The requested resource '%s' could not be found.", r.URL.Path)
			return
		}

		log.Println("Found file at", path)

		templatePath := FindTemplateRuntime(path)
		if templatePath == "" {
			log.Println("No template found, using default")
			//without a template, things don't work, so just chuck something simple in there
			w.Write([]byte(renderer.GenerateSingleFile(string(file), "<!doctype html><body>{{slot}}</body>", path)))
			return
		}

		template, err := os.ReadFile(templatePath)
		if err != nil {
			log.Printf("Error reading template at %s: %v\n", templatePath, err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "500 Internal Server Error: Could not read template")
			return
		}

		w.Write([]byte(renderer.GenerateSingleFile(string(file), string(template), path)))
	})

	fmt.Println("Server starting on port 8080...")
	http.ListenAndServe(":8080", nil)
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

func getContentType(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".html":
		return "text/html"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
