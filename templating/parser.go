package templating

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	aliasPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type SyntaxError struct {
	TemplatePath string
	Position     Position
	Message      string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("template %s:%d:%d: %s", e.TemplatePath, e.Position.Line, e.Position.Column, e.Message)
}

type parserState struct {
	path      string
	source    string
	offset    int
	line      int
	column    int
	slotCount int
}

func Parse(path, source string) (*Template, error) {
	p := &parserState{path: path, source: source, line: 1, column: 1}
	nodes, closed, err := p.parseNodes(0)
	if err != nil {
		return nil, err
	}
	if closed {
		return nil, p.errorAt(p.position(), "unexpected {{/each}}")
	}
	if p.slotCount == 0 {
		return nil, p.errorAt(p.position(), "template must contain exactly one {{slot}}")
	}
	return &Template{Path: path, Nodes: nodes}, nil
}

func (p *parserState) parseNodes(depth int) ([]Node, bool, error) {
	var nodes []Node
	for p.offset < len(p.source) {
		start := strings.Index(p.source[p.offset:], "{{")
		if start < 0 {
			position := p.position()
			text := p.source[p.offset:]
			p.advance(text)
			if text != "" {
				nodes = append(nodes, &TextNode{Text: text, Position: position})
			}
			return nodes, false, nil
		}
		if start > 0 {
			position := p.position()
			text := p.source[p.offset : p.offset+start]
			p.advance(text)
			nodes = append(nodes, &TextNode{Text: text, Position: position})
		}

		position := p.position()
		end := strings.Index(p.source[p.offset+2:], "}}")
		if end < 0 {
			return nil, false, p.errorAt(position, "unclosed template directive")
		}
		end += p.offset + 2
		raw := p.source[p.offset : end+2]
		body := strings.TrimSpace(p.source[p.offset+2 : end])
		p.advance(raw)

		switch {
		case body == "/each":
			if depth == 0 {
				return nil, false, p.errorAt(position, "unexpected {{/each}}")
			}
			return nodes, true, nil
		case body == "#each" || strings.HasPrefix(body, "#each "):
			each, err := p.parseEachHeader(body, position)
			if err != nil {
				return nil, false, err
			}
			children, closed, err := p.parseNodes(depth + 1)
			if err != nil {
				return nil, false, err
			}
			if !closed {
				return nil, false, p.errorAt(position, "unclosed each block")
			}
			each.Children = children
			nodes = append(nodes, &each)
		case body == "slot":
			if depth > 0 {
				return nil, false, p.errorAt(position, "{{slot}} cannot appear inside an each block")
			}
			p.slotCount++
			if p.slotCount > 1 {
				return nil, false, p.errorAt(position, "template must contain exactly one {{slot}}")
			}
			nodes = append(nodes, &SlotNode{Position: position})
		default:
			value, ok := parseValue(body)
			if !ok {
				return nil, false, p.errorAt(position, fmt.Sprintf("unknown directive %q", body))
			}
			value.Position = position
			nodes = append(nodes, &value)
		}
	}
	return nodes, false, nil
}

func (p *parserState) parseEachHeader(body string, position Position) (EachNode, error) {
	words := strings.Fields(body)
	if len(words) < 4 || words[0] != "#each" || words[2] != "as" {
		return EachNode{}, p.errorAt(position, "each header must be: {{#each <source> as <alias> [sort <property> [asc|desc]]}}")
	}
	if len(words) != 4 && len(words) != 6 && len(words) != 7 {
		return EachNode{}, p.errorAt(position, "extra or unrecognized words in each header")
	}
	if !validSource(words[1]) {
		return EachNode{}, p.errorAt(position, fmt.Sprintf("invalid collection source %q", words[1]))
	}
	if !aliasPattern.MatchString(words[3]) {
		return EachNode{}, p.errorAt(position, fmt.Sprintf("invalid alias %q", words[3]))
	}

	node := EachNode{Source: words[1], Alias: words[3], SortDirection: SortAscending, Position: position}
	if len(words) >= 6 {
		if words[4] != "sort" || !validProperty(words[5]) {
			return EachNode{}, p.errorAt(position, "invalid sort clause")
		}
		node.SortProperty = words[5]
	}
	if len(words) == 7 {
		switch words[6] {
		case "asc":
		case "desc":
			node.SortDirection = SortDescending
		default:
			return EachNode{}, p.errorAt(position, fmt.Sprintf("invalid sort direction %q", words[6]))
		}
	}
	return node, nil
}

func parseValue(body string) (ValueNode, bool) {
	if strings.TrimSpace(body) != body || strings.Count(body, ".") != 1 {
		return ValueNode{}, false
	}
	parts := strings.Split(body, ".")
	if !aliasPattern.MatchString(parts[0]) || !validProperty(parts[1]) {
		return ValueNode{}, false
	}
	return ValueNode{Path: parts}, true
}

func validProperty(property string) bool {
	return property != "" && !strings.ContainsAny(property, ".{} \t\r\n")
}

func validSource(source string) bool {
	if source == "." || source == "meta.headings" {
		return true
	}
	return strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../")
}

func (p *parserState) position() Position {
	return Position{Offset: p.offset, Line: p.line, Column: p.column}
}

func (p *parserState) advance(value string) {
	for len(value) > 0 {
		r, size := utf8.DecodeRuneInString(value)
		value = value[size:]
		p.offset += size
		if r == '\n' {
			p.line++
			p.column = 1
		} else {
			p.column++
		}
	}
}

func (p *parserState) errorAt(position Position, message string) error {
	return &SyntaxError{TemplatePath: p.path, Position: position, Message: message}
}
