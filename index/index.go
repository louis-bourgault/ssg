package index

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/louis-bourgault/ssg/types"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
)

type ProjectIndex struct {
	Directories map[string]*DirectoryIndex
}

func (p *ProjectIndex) Finalise() {
	//write everything to disk
	for dirPath, dirIndex := range p.Directories {
		//log.Println("Finalising directory of ", dirPath, "with", len(dirIndex.Files), "files")
		jsonData, err := json.Marshal(dirIndex)
		if err != nil {
			log.Println("Error marshalling index for directory", dirPath, ":", err)
			continue
		}
		//fmt.Println(string(jsonData))
		//write to a file called .index.json in the directory that it's indexing
		indexFilePath := filepath.Join(dirPath, ".index.json")
		err = os.WriteFile(indexFilePath, jsonData, 0644)
		if err != nil {
			log.Println("Error writing index file for directory", dirPath, ":", err)
		}
	}
	jsonData, err := json.Marshal(p)
	if err != nil {
		log.Println("Error marshalling project index:", err)
		return
	}
	err = os.WriteFile(".projectindex.json", jsonData, 0644)
	if err != nil {
		log.Println("Error writing project index file:", err)
	}
}

func (p *ProjectIndex) AddFile(file types.File, content string) {
	//log.Println("Recieved file, ", file.OriginalPath)
	directory := filepath.Dir(file.OriginalPath)
	dirIndex, exists := p.Directories[directory]
	if !exists {
		dirIndex = NewDirectoryIndex(directory)
		p.Directories[directory] = dirIndex
	}
	dirIndex.AddFile(file, content)
}

type DirectoryIndex struct {
	Path       string            `json:"path"`
	Properties map[string]string `json:"properties"`
	Files      []FileIndex       `json:"files"`
}

type FileIndex struct {
	File       types.File     `json:"file"`
	Properties map[string]any `json:"properties"`
}

func NewDirectoryIndex(path string) *DirectoryIndex {
	return &DirectoryIndex{
		Path:       path,
		Properties: make(map[string]string),
		Files:      []FileIndex{},
	}
}

func (d *DirectoryIndex) AddFile(file types.File, content string) {
	//log.Println("Recieved file, ", file.OriginalPath)
	markdown := goldmark.New(
		goldmark.WithExtensions(
			meta.Meta,
		),
	)

	var buf bytes.Buffer
	context := parser.NewContext()
	if err := markdown.Convert([]byte(content), &buf, parser.WithContext(context)); err != nil {
		log.Println("Error converting markdown for", file.OriginalPath, ":", err)
		d.Files = append(d.Files, FileIndex{
			File:       file,
			Properties: make(map[string]any),
		})
		return
	}
	metaData := meta.Get(context)

	if len(d.Files) == 0 {
		for key, value := range metaData {
			d.Properties[key] = DetectType(value)
		}
	} else {
		for key, value := range metaData {
			propType, exists := d.Properties[key]
			if !exists {
				delete(metaData, key)
				continue
			}

			actualType := DetectType(value)
			if actualType != propType {
				log.Println("Property", key, "has different types across files:", propType, "and", actualType)
				if propType != "string" {
					for i := range d.Files {
						d.Files[i].Properties[key] = fmt.Sprintf("%v", d.Files[i].Properties[key])
					}
					d.Properties[key] = "string"
				}
				metaData[key] = fmt.Sprintf("%v", value)
			}
		}

		for prop := range d.Properties {
			if _, has := metaData[prop]; !has {
				//log.Println("File", file.OriginalPath, "is missing property", prop)
				delete(d.Properties, prop)
			}
		}
	}

	d.Files = append(d.Files, FileIndex{
		File:       file,
		Properties: metaData,
	})

}

func DetectType(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case int, int64, float64:
		return "number"
	case bool:
		return "boolean"
	case time.Time:
		return "date"
	case []any:
		return "array" //arrays are not supported yet
	default:
		return "string" // Fallback
	}
}
