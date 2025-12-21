package index

import (
	"log"
)

type DirectoryIndex struct {
	path       string
	properties map[string]string
	files      []FileIndex
}

type FileIndex struct {
	path       string
	properties map[string]string
}

func NewDirectoryIndex(path string) DirectoryIndex {
	return DirectoryIndex{
		path: path,
	}
}

func (d *DirectoryIndex) AddFile(path string, content string) {
	log.Println("Recieved file, ", path)
	
}
