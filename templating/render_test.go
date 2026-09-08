package templating

import (
	"strings"
	"testing"

	"github.com/louis-bourgault/ssg/index"
	"github.com/louis-bourgault/ssg/types"
)

func TestRenderMetadataScalarsEscapingAndErrors(t *testing.T) {
	page := Page{
		SourcePath:   "routes/about.md",
		RenderedHTML: "<h1>Trusted</h1> {{meta.literal}}",
		Meta: map[string]any{
			"title":  `<b>A & B</b>`,
			"count":  12.5,
			"draft":  true,
			"tags":   []any{"go"},
			"author": map[any]any{"name": "Ada"},
		},
	}
	project := index.NewProjectIndex()

	got := renderForTest(t, `{{meta.title}}/{{meta.count}}/{{meta.draft}}/{{slot}}`, page, project)
	if got != `&lt;b&gt;A &amp; B&lt;/b&gt;/12.5/true/<h1>Trusted</h1> {{meta.literal}}` {
		t.Fatalf("render = %q", got)
	}

	for _, test := range []struct{ property, message string }{
		{"missing", `page routes/about.md has no metadata property "missing"`},
		{"tags", "expected a scalar, got an array"},
		{"author", "expected a scalar, got an object"},
	} {
		parsed, err := Parse("routes/template.html", "{{meta."+test.property+"}}{{slot}}")
		if err != nil {
			t.Fatal(err)
		}
		_, err = Render(parsed, RenderContext{CurrentPage: page, Project: project})
		if err == nil || !strings.Contains(err.Error(), test.message) {
			t.Errorf("%s error = %v", test.property, err)
		}
	}
}

func TestRenderAliasesReservedPropertiesAndUnicodePreview(t *testing.T) {
	project := index.NewProjectIndex()
	addTestPage(t, project, Page{SourcePath: "routes/posts/café.md", OutputURL: "/posts/café/", Filename: "café.md", PlainText: "é猫🙂more", Meta: map[string]any{"title": "Café"}})
	current := Page{SourcePath: "routes/index.md", RenderedHTML: "content", Meta: map[string]any{}}
	got := renderForTest(t, `{{#each ./posts as post}}{{post.title}}|{{post._url}}|{{post._filename}}|{{post._path}}|{{post._preview3}}{{/each}}{{slot}}`, current, project)
	want := `Café|/posts/café/|café.md|posts/café.md|é猫🙂...content`
	if got != want {
		t.Fatalf("render = %q, want %q", got, want)
	}
}

func TestAliasMayShadowMetaWithinItsScope(t *testing.T) {
	project := index.NewProjectIndex()
	addTestPage(t, project, Page{SourcePath: "routes/posts/one.md", Meta: map[string]any{"title": "Post"}})
	current := Page{SourcePath: "routes/index.md", Meta: map[string]any{"title": "Current"}}
	got := renderForTest(t, `{{meta.title}}/{{#each ./posts as meta}}{{meta.title}}{{/each}}/{{meta.title}}{{slot}}`, current, project)
	if got != "Current/Post/Current" {
		t.Fatalf("render = %q", got)
	}
}

func TestRenderDirectoryResolutionAndOrdering(t *testing.T) {
	project := index.NewProjectIndex()
	for _, page := range []Page{
		{SourcePath: "routes/blog/z.md", Filename: "z.md", Meta: map[string]any{"title": "current-z"}},
		{SourcePath: "routes/blog/a.md", Filename: "a.md", Meta: map[string]any{"title": "current-a"}},
		{SourcePath: "routes/blog/posts/b.md", Filename: "b.md", Meta: map[string]any{"title": "child"}},
		{SourcePath: "routes/posts/p.md", Filename: "p.md", Meta: map[string]any{"title": "parent"}},
	} {
		addTestPage(t, project, page)
	}
	current := Page{SourcePath: "routes/blog/index.md", Meta: map[string]any{}, RenderedHTML: "!"}
	got := renderForTest(t, `{{#each . as x}}{{x.title}},{{/each}}|{{#each ./posts as x}}{{x.title}}{{/each}}|{{#each ../posts as x}}{{x.title}}{{/each}}{{slot}}`, current, project)
	if got != "current-a,current-z,|child|parent!" {
		t.Fatalf("render = %q", got)
	}

	parsed, err := Parse("routes/blog/template.html", `{{#each ../../outside as x}}{{x.title}}{{/each}}{{slot}}`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Render(parsed, RenderContext{CurrentPage: current, Project: project})
	if err == nil || !strings.Contains(err.Error(), "escapes the routes directory") {
		t.Fatalf("escape error = %v", err)
	}
}

func TestRenderStableSortingAndSortErrors(t *testing.T) {
	project := index.NewProjectIndex()
	for _, page := range []Page{
		{SourcePath: "routes/posts/c.md", Meta: map[string]any{"title": "C", "rank": 2, "date": "2026-01-01"}},
		{SourcePath: "routes/posts/a.md", Meta: map[string]any{"title": "A", "rank": 1, "date": "2026-01-01"}},
		{SourcePath: "routes/posts/b.md", Meta: map[string]any{"title": "B", "rank": 2, "date": "2025-01-01"}},
	} {
		addTestPage(t, project, page)
	}
	current := Page{SourcePath: "routes/index.md", Meta: map[string]any{}}
	asc := renderForTest(t, `{{#each ./posts as p sort rank}}{{p.title}}{{/each}}{{slot}}`, current, project)
	desc := renderForTest(t, `{{#each ./posts as p sort rank desc}}{{p.title}}{{/each}}{{slot}}`, current, project)
	if asc != "ABC" || desc != "BCA" {
		t.Fatalf("sort asc/desc = %q/%q", asc, desc)
	}

	missing := Page{SourcePath: "routes/missing/x.md", Meta: map[string]any{"title": "X"}}
	addTestPage(t, project, missing)
	assertRenderError(t, project, current, `{{#each ./missing as p sort rank}}{{p.title}}{{/each}}{{slot}}`, `page routes/missing/x.md has no metadata property "rank"`)

	addTestPage(t, project, Page{SourcePath: "routes/mixed/a.md", Meta: map[string]any{"value": "1"}})
	addTestPage(t, project, Page{SourcePath: "routes/mixed/b.md", Meta: map[string]any{"value": 2}})
	assertRenderError(t, project, current, `{{#each ./mixed as p sort value}}{{p.value}}{{/each}}{{slot}}`, "incompatible value types")
}

func TestRenderHeadingsInDocumentOrder(t *testing.T) {
	page := Page{SourcePath: "routes/index.md", Headings: []Heading{{Level: 2, Text: "A & B", ID: "a-b"}, {Level: 6, Text: "Last", ID: "last"}}}
	got := renderForTest(t, `{{#each meta.headings as heading}}{{heading.level}}:{{heading.text}}:#{{heading.id}};{{/each}}{{slot}}`, page, index.NewProjectIndex())
	if got != "2:A &amp; B:#a-b;6:Last:#last;" {
		t.Fatalf("render = %q", got)
	}
}

func TestCacheCompilesEachPathOnce(t *testing.T) {
	cache := NewCache()
	first, err := cache.Compile("routes/template.html", "{{slot}}")
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Compile("routes/template.html", "changed {{slot}}")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || cache.CompileCount() != 1 {
		t.Fatalf("cache returned %p/%p with count %d", first, second, cache.CompileCount())
	}
}

func renderForTest(t *testing.T, source string, page Page, project *index.ProjectIndex) string {
	t.Helper()
	parsed, err := Parse("routes/template.html", source)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Render(parsed, RenderContext{CurrentPage: page, Project: project})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func addTestPage(t *testing.T, project *index.ProjectIndex, page Page) {
	t.Helper()
	if page.Filename == "" {
		parts := strings.Split(page.SourcePath, "/")
		page.Filename = parts[len(parts)-1]
	}
	if page.Meta == nil {
		page.Meta = map[string]any{}
	}
	if err := project.AddPage(types.File{OriginalPath: page.SourcePath, Type: "md"}, page); err != nil {
		t.Fatal(err)
	}
}

func assertRenderError(t *testing.T, project *index.ProjectIndex, page Page, source, message string) {
	t.Helper()
	parsed, err := Parse("routes/template.html", source)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Render(parsed, RenderContext{CurrentPage: page, Project: project})
	if err == nil || !strings.Contains(err.Error(), message) {
		t.Fatalf("error = %v, want %q", err, message)
	}
}
