package templating

import (
	"fmt"
	"html"
	"path/filepath"
	"sort"
	"strings"
)

type scopeValue struct {
	page    *Page
	heading *Heading
}

type collectionEntry struct {
	value scopeValue
	path  string
}

type renderState struct {
	template *Template
	context  RenderContext
	scopes   []map[string]scopeValue
}

func Render(template *Template, context RenderContext) (string, error) {
	if template == nil {
		return "", fmt.Errorf("template is nil")
	}
	state := &renderState{template: template, context: context}
	var output strings.Builder
	if err := state.renderNodes(&output, template.Nodes); err != nil {
		return "", err
	}
	return output.String(), nil
}

func (r *renderState) renderNodes(output *strings.Builder, nodes []Node) error {
	for _, rawNode := range nodes {
		switch node := rawNode.(type) {
		case *TextNode:
			output.WriteString(node.Text)
		case *SlotNode:
			output.WriteString(r.context.CurrentPage.RenderedHTML)
		case *ValueNode:
			value, err := r.resolveValue(node.Path)
			if err != nil {
				return r.nodeError(node.Position, err)
			}
			formatted, err := scalarString(strings.Join(node.Path, "."), value)
			if err != nil {
				return r.nodeError(node.Position, err)
			}
			output.WriteString(html.EscapeString(formatted))
		case *EachNode:
			if err := r.renderEach(output, *node); err != nil {
				return err
			}
		default:
			return fmt.Errorf("template %s: unsupported AST node %T", r.template.Path, rawNode)
		}
	}
	return nil
}

func (r *renderState) renderEach(output *strings.Builder, node EachNode) error {
	entries, err := r.collection(node)
	if err != nil {
		return r.nodeError(node.Position, err)
	}
	if node.SortProperty != "" {
		if err := r.sortEntries(entries, node); err != nil {
			return r.nodeError(node.Position, err)
		}
	}
	for _, entry := range entries {
		r.scopes = append(r.scopes, map[string]scopeValue{node.Alias: entry.value})
		err := r.renderNodes(output, node.Children)
		r.scopes = r.scopes[:len(r.scopes)-1]
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *renderState) collection(node EachNode) ([]collectionEntry, error) {
	if node.Source == "meta.headings" {
		entries := make([]collectionEntry, 0, len(r.context.CurrentPage.Headings))
		for i := range r.context.CurrentPage.Headings {
			entries = append(entries, collectionEntry{value: scopeValue{heading: &r.context.CurrentPage.Headings[i]}, path: fmt.Sprintf("%08d", i)})
		}
		return entries, nil
	}
	if r.context.Project == nil {
		return nil, fmt.Errorf("project index is unavailable")
	}
	directory, err := r.resolveDirectory(node.Source)
	if err != nil {
		return nil, err
	}
	directoryIndex, exists := r.context.Project.Directories[directory]
	if !exists {
		return nil, fmt.Errorf("directory %q is not indexed", directory)
	}
	entries := make([]collectionEntry, 0, len(directoryIndex.Files))
	for i := range directoryIndex.Files {
		page := &directoryIndex.Files[i].Page
		entries = append(entries, collectionEntry{value: scopeValue{page: page}, path: filepath.ToSlash(page.SourcePath)})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	return entries, nil
}

func (r *renderState) resolveDirectory(source string) (string, error) {
	currentDirectory := filepath.Dir(filepath.FromSlash(r.context.CurrentPage.SourcePath))
	candidate := currentDirectory
	if source != "." {
		candidate = filepath.Join(currentDirectory, filepath.FromSlash(source))
	}
	root, err := filepath.Abs(filepath.FromSlash(r.context.Project.RoutesDir))
	if err != nil {
		return "", fmt.Errorf("resolve routes directory: %w", err)
	}
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve collection path %q: %w", source, err)
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil {
		return "", fmt.Errorf("resolve collection path %q: %w", source, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("collection path %q escapes the routes directory", source)
	}
	return filepath.ToSlash(filepath.Clean(candidate)), nil
}

func (r *renderState) sortEntries(entries []collectionEntry, node EachNode) error {
	values := make([]sortableValue, len(entries))
	kind := ""
	for i, entry := range entries {
		value, err := r.entryProperty(entry.value, node.SortProperty)
		if err != nil {
			return err
		}
		sortable, err := makeSortable(node.Alias+"."+node.SortProperty, value)
		if err != nil {
			return err
		}
		if kind != "" && sortable.kind != kind {
			return fmt.Errorf("cannot sort property %q: incompatible value types %s and %s", node.SortProperty, kind, sortable.kind)
		}
		kind = sortable.kind
		values[i] = sortable
	}

	type paired struct {
		entry collectionEntry
		value sortableValue
	}
	pairs := make([]paired, len(entries))
	for i := range entries {
		pairs[i] = paired{entry: entries[i], value: values[i]}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		comparison := compareSortable(pairs[i].value, pairs[j].value)
		if comparison == 0 {
			return pairs[i].entry.path < pairs[j].entry.path
		}
		if node.SortDirection == SortDescending {
			return comparison > 0
		}
		return comparison < 0
	})
	for i := range pairs {
		entries[i] = pairs[i].entry
	}
	return nil
}

func compareSortable(left, right sortableValue) int {
	if left.kind == "number" {
		return left.number.Cmp(right.number)
	}
	return strings.Compare(left.text, right.text)
}

func (r *renderState) resolveValue(path []string) (any, error) {
	for i := len(r.scopes) - 1; i >= 0; i-- {
		if value, exists := r.scopes[i][path[0]]; exists {
			return r.entryProperty(value, path[1])
		}
	}
	if path[0] == "meta" {
		if path[1] == "headings" {
			return r.context.CurrentPage.Headings, nil
		}
		value, exists := r.context.CurrentPage.Meta[path[1]]
		if !exists {
			return nil, fmt.Errorf("page %s has no metadata property %q", r.context.CurrentPage.SourcePath, path[1])
		}
		return value, nil
	}
	return nil, fmt.Errorf("unknown template alias %q", path[0])
}

func (r *renderState) entryProperty(value scopeValue, property string) (any, error) {
	if value.page != nil {
		return pageProperty(r.context.Project, value.page, property)
	}
	if value.heading != nil {
		return headingProperty(*value.heading, property)
	}
	return nil, fmt.Errorf("invalid collection value")
}

func (r *renderState) nodeError(position Position, err error) error {
	return fmt.Errorf("template %s:%d:%d: %w", r.template.Path, position.Line, position.Column, err)
}

// Cache compiles each template path once. A cache is intended to live for one build.
type Cache struct {
	templates map[string]*Template
	count     int
}

func NewCache() *Cache { return &Cache{templates: make(map[string]*Template)} }

func (c *Cache) Compile(path, source string) (*Template, error) {
	if template, exists := c.templates[path]; exists {
		return template, nil
	}
	template, err := Parse(path, source)
	if err != nil {
		return nil, err
	}
	c.templates[path] = template
	c.count++
	return template, nil
}

func (c *Cache) CompileCount() int { return c.count }
