package renderer

import (
	"strings"
	"testing"

	"github.com/louis-bourgault/ssg/index"
)

func TestProcessHTMLRewritesOnlyURLAttributes(t *testing.T) {
	input := `<a href='./guide.md?view=full#part' data-href="./unchanged.md">Guide</a>` +
		`<a href="ftp://example.com/file.md">External</a>` +
		`<script>const example = 'href="./not-a-link.md"';</script>`

	got, err := processHTML(input, "routes/docs/index.md", true, false)
	if err != nil {
		t.Fatalf("processHTML returned an error: %v", err)
	}

	if !strings.Contains(got, `href="/docs/guide/?view=full#part"`) {
		t.Errorf("relative Markdown URL was not rewritten correctly: %s", got)
	}
	if !strings.Contains(got, `data-href="./unchanged.md"`) {
		t.Errorf("non-URL attribute was changed: %s", got)
	}
	if !strings.Contains(got, `href="ftp://example.com/file.md"`) {
		t.Errorf("external URL was changed: %s", got)
	}
	if !strings.Contains(got, `'href="./not-a-link.md"'`) {
		t.Errorf("script text was changed: %s", got)
	}
}

func TestProcessHTMLLeavesSVGImageUnchanged(t *testing.T) {
	input := `<img src="/icon.svg" alt="icon">`
	got, err := processHTML(input, "routes/index.md", true, true)
	if err != nil {
		t.Fatalf("processHTML returned an error: %v", err)
	}
	if got != input {
		t.Fatalf("SVG image changed:\n got: %s\nwant: %s", got, input)
	}
}

func TestGenerateSingleFileValidatesTemplateSlot(t *testing.T) {
	projectIndex := index.NewProjectIndex()
	tests := []string{
		"<html><body>missing slot</body></html>",
		"{{slot}}<hr>{{slot}}",
	}
	for _, template := range tests {
		if _, err := GenerateSingleFile("# Page", template, "routes/index.md", projectIndex); err == nil {
			t.Errorf("GenerateSingleFile accepted invalid template %q", template)
		}
	}
}
