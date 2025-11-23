package dev

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for dev
	CheckOrigin: func(r *http.Request) bool { return true },
}

func DevWS(w http.ResponseWriter, r *http.Request) {
	log.Println("Ws Request on", r.URL)

	urlPath := strings.TrimPrefix(r.URL.Path, "/_devws")
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}

	mainFilePath, err := FindFile(urlPath)
	if err != nil {
		log.Println("Could not find file to watch:", urlPath)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	templatePath := FindTemplateRuntime(mainFilePath)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Println("Watcher error:", err)
		return
	}
	defer watcher.Close()

	err = watcher.Add(mainFilePath)
	if err != nil {
		log.Println("Error watching file:", err)
		return
	}

	if templatePath != "" {
		err := watcher.Add(templatePath)
		if err != nil {
			log.Println("Error watching template:", err)
			return
		}
	}

	fmt.Println("we'll be watching", mainFilePath, templatePath)

	done := make(chan bool)
	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				close(done)
				return
			}
		}
	}()

	log.Println("watching the file ", mainFilePath)

	for {
		select {
		case <-done:
			log.Println("Client disconnected")
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Println("Modified file:", event.Name)

				err := conn.WriteMessage(websocket.TextMessage, []byte("reload"))
				if err != nil {
					log.Println("Write error:", err)
					return
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("Watcher error:", err)
		}
	}
}
