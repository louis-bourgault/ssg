package index

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/louis-bourgault/ssg/sitepath"
	"github.com/louis-bourgault/ssg/types"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkHTML "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	nethtml "golang.org/x/net/html"
)

type Heading struct {
	Level int
	Text  string
	ID    string
}

type Page struct {
	SourcePath   string
	OutputURL    string
	Filename     string
	Meta         map[string]any
	PlainText    string
	Headings     []Heading
	RenderedHTML string
}

type ProjectIndex struct {
	RoutesDir   string
	Directories map[string]*DirectoryIndex
	Pages       map[string]*Page
}

func NewProjectIndex() *ProjectIndex {
	return NewProjectIndexForRoutes("routes")
}

func NewProjectIndexForRoutes(routesDir string) *ProjectIndex {
	return &ProjectIndex{
		RoutesDir:   cleanPath(routesDir),
		Directories: map[string]*DirectoryIndex{},
		Pages:       map[string]*Page{},
	}
}

func BuildFromDirectory(rootPath string) (*ProjectIndex, error) {
	projectIndex := NewProjectIndexForRoutes(rootPath)
	err := filepath.WalkDir(rootPath, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			projectIndex.AddDirectory(filePath)
			return nil
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read index source %q: %w", filePath, err)
		}
		file := types.File{OriginalPath: cleanPath(filePath), Type: "md"}
		return projectIndex.AddFile(file, string(content))
	})
	if err != nil {
		return nil, fmt.Errorf("build project index: %w", err)
	}
	projectIndex.Sort()
	return projectIndex, nil
}

func (p *ProjectIndex) AddDirectory(path string) {
	path = cleanPath(path)
	if _, exists := p.Directories[path]; !exists {
		p.Directories[path] = NewDirectoryIndex(path)
	}
}

func (p *ProjectIndex) AddFile(file types.File, content string) error {
	file.OriginalPath = cleanPath(file.OriginalPath)
	page, err := ParsePage(p.RoutesDir, file.OriginalPath, content)
	if err != nil {
		return err
	}
	return p.AddPage(file, page)
}

func (p *ProjectIndex) AddPage(file types.File, page Page) error {
	for property := range page.Meta {
		if strings.HasPrefix(property, "_") {
			return fmt.Errorf("index page %s: frontmatter property %q is reserved by the SSG", page.SourcePath, property)
		}
		if property == "headings" {
			return fmt.Errorf("index page %s: frontmatter property %q is reserved for generated headings", page.SourcePath, property)
		}
	}
	directory := cleanPath(filepath.Dir(page.SourcePath))
	p.AddDirectory(directory)
	dirIndex := p.Directories[directory]
	dirIndex.addPage(file, page)
	p.Pages[cleanPath(page.SourcePath)] = &dirIndex.Files[len(dirIndex.Files)-1].Page
	p.Sort()
	return nil
}

func (p *ProjectIndex) Page(sourcePath string) (*Page, bool) {
	page, ok := p.Pages[cleanPath(sourcePath)]
	return page, ok
}

func (p *ProjectIndex) Sort() {
	for _, directory := range p.Directories {
		sort.SliceStable(directory.Files, func(i, j int) bool {
			return cleanPath(directory.Files[i].Page.SourcePath) < cleanPath(directory.Files[j].Page.SourcePath)
		})
		for i := range directory.Files {
			p.Pages[cleanPath(directory.Files[i].Page.SourcePath)] = &directory.Files[i].Page
		}
	}
}

type DirectoryIndex struct {
	Path       string            `json:"path"`
	Properties map[string]string `json:"properties"`
	Files      []FileIndex       `json:"files"`
}

type FileIndex struct {
	File       types.File     `json:"file"`
	Properties map[string]any `json:"properties"`
	Page       Page           `json:"page"`
}

func NewDirectoryIndex(path string) *DirectoryIndex {
	return &DirectoryIndex{Path: cleanPath(path), Properties: make(map[string]string), Files: []FileIndex{}}
}

func (d *DirectoryIndex) addPage(file types.File, page Page) {
	if len(d.Files) == 0 {
		for key, value := range page.Meta {
			d.Properties[key] = DetectType(value)
		}
	} else {
		for property, propertyType := range d.Properties {
			value, exists := page.Meta[property]
			if !exists || DetectType(value) != propertyType {
				delete(d.Properties, property)
			}
		}
	}
	d.Files = append(d.Files, FileIndex{File: file, Properties: page.Meta, Page: page})
}

func DetectType(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return "number"
	case bool:
		return "boolean"
	case time.Time:
		return "date"
	case []any, []string:
		return "array"
	case map[any]any, map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

// ParsePage parses and renders a Markdown page once, retaining all data needed
// by subsequent template references.
func ParsePage(routesDir, sourcePath, content string) (Page, error) {
	markdown := goldmark.New(
		goldmark.WithExtensions(extension.GFM, meta.Meta),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(goldmarkHTML.WithHardWraps(), goldmarkHTML.WithXHTML(), goldmarkHTML.WithUnsafe()),
	)
	source := []byte(content)
	context := parser.NewContext()
	document := markdown.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	var rendered bytes.Buffer
	if err := markdown.Renderer().Render(&rendered, source, document); err != nil {
		return Page{}, fmt.Errorf("render Markdown %q: %w", sourcePath, err)
	}

	parsedMetadata, err := meta.TryGet(context)
	if err != nil {
		return Page{}, fmt.Errorf("parse frontmatter for %q: %w", sourcePath, err)
	}
	metadata := make(map[string]any)
	for key, value := range parsedMetadata {
		metadata[key] = value
	}
	outputURL, err := sitepath.PrettyURL(routesDir, sourcePath)
	if err != nil {
		return Page{}, fmt.Errorf("index page %q: %w", sourcePath, err)
	}
	return Page{
		SourcePath:   cleanPath(sourcePath),
		OutputURL:    outputURL,
		Filename:     filepath.Base(sourcePath),
		Meta:         metadata,
		PlainText:    extractPlainText(rendered.String()),
		Headings:     collectHeadings(document, source),
		RenderedHTML: rendered.String(),
	}, nil
}

func collectHeadings(document ast.Node, source []byte) []Heading {
	var headings []Heading
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || node.Kind() != ast.KindHeading {
			return ast.WalkContinue, nil
		}
		heading := node.(*ast.Heading)
		id := ""
		if value, ok := heading.AttributeString("id"); ok {
			switch value := value.(type) {
			case []byte:
				id = string(value)
			case string:
				id = value
			}
		}
		headings = append(headings, Heading{Level: heading.Level, Text: inlinePlainText(heading, source), ID: id})
		return ast.WalkSkipChildren, nil
	})
	return headings
}

func inlinePlainText(parent ast.Node, source []byte) string {
	var result strings.Builder
	_ = ast.Walk(parent, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || node == parent {
			return ast.WalkContinue, nil
		}
		switch node := node.(type) {
		case *ast.Text:
			result.Write(node.Value(source))
			if node.SoftLineBreak() || node.HardLineBreak() {
				result.WriteByte(' ')
			}
		case *ast.String:
			result.Write(node.Value)
		case *ast.RawHTML:
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return stdhtml.UnescapeString(result.String())
}

func extractPlainText(documentText string) string {
	tokenizer := nethtml.NewTokenizer(strings.NewReader(documentText))
	var words []string
	for {
		switch tokenizer.Next() {
		case nethtml.ErrorToken:
			return strings.Join(words, " ")
		case nethtml.TextToken:
			words = append(words, strings.Fields(tokenizer.Token().Data)...)
		}
	}
}

func cleanPath(path string) string { return filepath.ToSlash(filepath.Clean(path)) }
