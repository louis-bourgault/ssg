package index

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/louis-bourgault/ssg/types"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
)

type ProjectIndex struct {
	Directories map[string]*DirectoryIndex
}

func NewProjectIndex() *ProjectIndex {
	return &ProjectIndex{Directories: map[string]*DirectoryIndex{}}
}

func BuildFromDirectory(rootPath string) (*ProjectIndex, error) {
	projectIndex := NewProjectIndex()
	err := filepath.WalkDir(rootPath, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read index source %q: %w", filePath, err)
		}
		file := types.File{
			OriginalPath: filepath.ToSlash(filePath),
			Type:         "md",
		}
		if err := projectIndex.AddFile(file, string(content)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("build project index: %w", err)
	}

	return projectIndex, nil
}

func (p *ProjectIndex) AddFile(file types.File, content string) error {
	//log.Println("Recieved file, ", file.OriginalPath)
	directory := filepath.Dir(file.OriginalPath)
	dirIndex, exists := p.Directories[directory]
	if !exists {
		dirIndex = NewDirectoryIndex(directory)
		p.Directories[directory] = dirIndex
	}
	return dirIndex.AddFile(file, content)
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

func (d *DirectoryIndex) AddFile(file types.File, content string) error {
	//log.Println("Recieved file, ", file.OriginalPath)
	markdown := goldmark.New(
		goldmark.WithExtensions(
			meta.Meta,
		),
	)

	var buf bytes.Buffer
	context := parser.NewContext()
	if err := markdown.Convert([]byte(content), &buf, parser.WithContext(context)); err != nil {
		return fmt.Errorf("parse metadata for %q: %w", file.OriginalPath, err)
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

	return nil
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
