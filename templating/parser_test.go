package templating

import (
	"strings"
	"testing"
)

func TestParseSupportedSyntax(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		validate func(*testing.T, *Template)
	}{
		{"plain HTML around slot", "<main>{{slot}}</main>", func(t *testing.T, parsed *Template) {
			if len(parsed.Nodes) != 3 {
				t.Fatalf("got %d nodes, want 3", len(parsed.Nodes))
			}
		}},
		{"metadata", "{{meta.Title}}{{slot}}", func(t *testing.T, parsed *Template) {
			value := parsed.Nodes[0].(*ValueNode)
			if strings.Join(value.Path, ".") != "meta.Title" {
				t.Fatalf("path = %v", value.Path)
			}
		}},
		{"alias", "{{#each . as post}}{{post.title}}{{/each}}{{slot}}", func(t *testing.T, parsed *Template) {
			each := parsed.Nodes[0].(*EachNode)
			if each.Alias != "post" || len(each.Children) != 1 {
				t.Fatalf("unexpected each node: %#v", each)
			}
		}},
		{"sort default", "{{#each ./posts as post sort date}}{{post.date}}{{/each}}{{slot}}", sortDirectionTest(SortAscending)},
		{"sort ascending", "{{#each ./posts as post sort date asc}}{{post.date}}{{/each}}{{slot}}", sortDirectionTest(SortAscending)},
		{"sort descending", "{{#each ./posts as post sort date desc}}{{post.date}}{{/each}}{{slot}}", sortDirectionTest(SortDescending)},
		{"nested", "{{#each . as outer}}{{#each ./posts as inner}}{{outer.title}}/{{inner.title}}{{/each}}{{/each}}{{slot}}", func(t *testing.T, parsed *Template) {
			outer := parsed.Nodes[0].(*EachNode)
			if _, ok := outer.Children[0].(*EachNode); !ok {
				t.Fatalf("nested node = %T", outer.Children[0])
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := Parse("routes/template.html", test.source)
			if err != nil {
				t.Fatalf("Parse returned an error: %v", err)
			}
			test.validate(t, parsed)
		})
	}
}

func sortDirectionTest(want SortDirection) func(*testing.T, *Template) {
	return func(t *testing.T, parsed *Template) {
		each := parsed.Nodes[0].(*EachNode)
		if each.SortProperty != "date" || each.SortDirection != want {
			t.Fatalf("sort = %q/%v", each.SortProperty, each.SortDirection)
		}
	}
}

func TestParseRejectsInvalidTemplates(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		message string
	}{
		{"missing closing", "{{#each . as item}}{{item.Title}}{{slot}}", "{{slot}} cannot appear inside an each block"},
		{"unclosed block", "{{slot}}{{#each . as item}}", "unclosed each block"},
		{"unexpected closing", "{{slot}}\n{{/each}}", "unexpected {{/each}}"},
		{"invalid alias starts number", "{{#each . as 2items}}{{/each}}{{slot}}", "invalid alias"},
		{"invalid alias punctuation", "{{#each . as it-em}}{{/each}}{{slot}}", "invalid alias"},
		{"unknown directive", "{{#if thing}}{{slot}}", "unknown directive"},
		{"extra words", "{{#each . as item sort date desc extra}}{{/each}}{{slot}}", "extra or unrecognized words"},
		{"no slot", "<html></html>", "exactly one {{slot}}"},
		{"two slots", "{{slot}} x {{slot}}", "exactly one {{slot}}"},
		{"slot in block", "{{#each . as item}}{{slot}}{{/each}}", "inside an each block"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse("routes/template.html", test.source)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error = %v, want message containing %q", err, test.message)
			}
			if !strings.Contains(err.Error(), "template routes/template.html:") {
				t.Fatalf("error lacks template location: %v", err)
			}
		})
	}
}

func TestParseReportsRuneAwareLineAndColumn(t *testing.T) {
	_, err := Parse("routes/template.html", "é\n  {{wat}}\n{{slot}}")
	if err == nil {
		t.Fatal("Parse succeeded")
	}
	if !strings.Contains(err.Error(), "routes/template.html:2:3") {
		t.Fatalf("error = %v, want line 2 column 3", err)
	}
}
