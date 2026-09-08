// Package templating implements the deliberately small SSG template language.
package templating

import "github.com/louis-bourgault/ssg/index"

type Position struct {
	Offset int
	Line   int
	Column int
}

type Node interface {
	templateNode()
}

type TextNode struct {
	Text     string
	Position Position
}

func (TextNode) templateNode() {}

type SlotNode struct{ Position Position }

func (SlotNode) templateNode() {}

type ValueNode struct {
	Path     []string
	Position Position
}

func (ValueNode) templateNode() {}

type SortDirection int

const (
	SortAscending SortDirection = iota
	SortDescending
)

type EachNode struct {
	Source        string
	Alias         string
	SortProperty  string
	SortDirection SortDirection
	Children      []Node
	Position      Position
}

func (EachNode) templateNode() {}

type Template struct {
	Path  string
	Nodes []Node
}

type Page = index.Page
type Heading = index.Heading

type RenderContext struct {
	CurrentPage Page
	Project     *index.ProjectIndex
}
