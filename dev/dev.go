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
	case ".pdf":
		return "application/pdf"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".ico":
		return "image/x-icon"
	case ".doc", ".docx":
		return "application/msword"
	case ".ppt", ".pptx":
		return "application/vnd.ms-powerpoint"
	case ".xls", ".xlsx":
		return "application/vnd.ms-excel"
	case ".odt":
		return "application/vnd.oasis.opendocument.text"
	case ".odp":
		return "application/vnd.oasis.opendocument.presentation"
	case ".ods":
		return "application/vnd.oasis.opendocument.spreadsheet"
	case ".psd":
		return "image/vnd.adobe.photoshop"
	case ".ai":
		return "application/postscript"
	case ".afdesign":
		return "application/x-affinity-designer"
	case ".afphoto", ".af":
		return "application/x-affinity-photo"
	case ".afpub":
		return "application/x-affinity-publisher"
	case ".zip":
		return "application/zip"
	case ".webp":
		return "image/webp"
	case ".txt":
		return "text/plain"
	default:
		return "text/[unknown]"
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
