package index

import (
	"strings"
	"testing"

	"github.com/louis-bourgault/ssg/types"
)

func TestParsePageRetainsMetadataAndCollectsHeadings(t *testing.T) {
	content := "---\ntitle: Example\nextra: kept\n---\n# Hello *world* &amp; friends\n\n###### Last <b>bold</b>\n"
	page, err := ParsePage("routes", "routes/posts/example.md", content)
	if err != nil {
		t.Fatal(err)
	}
	if page.OutputURL != "/posts/example/" || page.Filename != "example.md" {
		t.Fatalf("derived page fields = %#v", page)
	}
	if page.Meta["title"] != "Example" || page.Meta["extra"] != "kept" {
		t.Fatalf("metadata = %#v", page.Meta)
	}
	if len(page.Headings) != 2 {
		t.Fatalf("headings = %#v", page.Headings)
	}
	if page.Headings[0] != (Heading{Level: 1, Text: "Hello world & friends", ID: "hello-world-amp-friends"}) {
		t.Fatalf("first heading = %#v", page.Headings[0])
	}
	if page.Headings[1].Level != 6 || page.Headings[1].Text != "Last bold" || page.Headings[1].ID != "last-bboldb" {
		t.Fatalf("last heading = %#v", page.Headings[1])
	}
	if !strings.Contains(page.RenderedHTML, `id="hello-world-amp-friends"`) || strings.Contains(page.PlainText, "<b>") {
		t.Fatalf("rendered/plain text = %q / %q", page.RenderedHTML, page.PlainText)
	}
}

func TestPageMetadataIsNotDeletedByDirectorySchema(t *testing.T) {
	project := NewProjectIndex()
	for _, file := range []struct{ path, content string }{
		{"routes/a.md", "---\ncommon: yes\nonly_a: kept\n---\nA"},
		{"routes/b.md", "---\ncommon: yes\nonly_b: kept\n---\nB"},
	} {
		if err := project.AddFile(types.File{OriginalPath: file.path, Type: "md"}, file.content); err != nil {
			t.Fatal(err)
		}
	}
	a, _ := project.Page("routes/a.md")
	b, _ := project.Page("routes/b.md")
	if a.Meta["only_a"] != "kept" || b.Meta["only_b"] != "kept" {
		t.Fatalf("page metadata was deleted: a=%#v b=%#v", a.Meta, b.Meta)
	}
	if _, exists := project.Directories["routes"].Properties["only_a"]; exists {
		t.Fatalf("non-common property remained in schema")
	}
}

func TestReservedFrontmatterIsRejected(t *testing.T) {
	project := NewProjectIndex()
	for _, content := range []string{"---\n_url: nope\n---\n", "---\nheadings: nope\n---\n"} {
		err := project.AddFile(types.File{OriginalPath: "routes/a.md", Type: "md"}, content)
		if err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("error = %v", err)
		}
	}
}
