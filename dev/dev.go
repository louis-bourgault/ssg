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

		path, err := FindFile(r.URL.Path)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "404 not found")
			return
		}

		file, err = os.ReadFile(path)

		if err != nil {
			fmt.Println("error reading file:", err)
			w.WriteHeader(http.StatusInternalServerError)
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

		w.Write([]byte(injectWSScript(renderer.GenerateSingleFile(string(file), string(template), path), r.URL.Path)))
	})

	http.HandleFunc("/_devws/", DevWS)

	fmt.Println("Server starting on port 8080...")
	http.ListenAndServe(":8080", nil)
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

func injectWSScript(original string, pageURL string) string {
	script := fmt.Sprintf(`<script>
		const wsUri = "ws://localhost:8080/_devws%s";
		const websocket = new WebSocket(wsUri);
		websocket.onopen = (event) => {
			console.log("Hot reloading system started")
		}
		websocket.onmessage = (event) => {
			if (event.data === "reload") {
				window.location.reload();
			}
		}
		websocket.onerror = (error) => {
			console.error(error)
		}
	</script>`, pageURL)

	if strings.Contains(original, "<head>") {
		return strings.Replace(original, "<head>", "<head>"+script, 1)
	}

	if strings.Contains(original, "<!DOCTYPE html>") || strings.Contains(original, "<!doctype html>") {
		return strings.Replace(original, ">", ">"+script, 1)
	}

	return "<head>" + script + "</head>" + original
}
