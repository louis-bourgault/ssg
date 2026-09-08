package renderer

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/louis-bourgault/ssg/image"
	"github.com/louis-bourgault/ssg/index"
	"github.com/louis-bourgault/ssg/sitepath"
	"github.com/louis-bourgault/ssg/templating"
	"github.com/louis-bourgault/ssg/types"
	"golang.org/x/net/html"
)

func GenerateSingleFile(content string, templateSource string, path string, project *index.ProjectIndex) (string, error) {
	routesDir := "routes"
	if project != nil && project.RoutesDir != "" {
		routesDir = project.RoutesDir
	}
	page, err := index.ParsePage(routesDir, path, content)
	if err != nil {
		return "", err
	}
	templatePath := filepath.ToSlash(filepath.Join(filepath.Dir(path), "template.html"))
	parsed, err := templating.Parse(templatePath, templateSource)
	if err != nil {
		return "", err
	}
	return GeneratePage(page, parsed, project)
}

func GeneratePage(page index.Page, template *templating.Template, project *index.ProjectIndex) (string, error) {
	rendered, err := templating.Render(template, templating.RenderContext{CurrentPage: page, Project: project})
	if err != nil {
		return "", err
	}
	routesDir := "routes"
	if project != nil && project.RoutesDir != "" {
		routesDir = project.RoutesDir
	}
	finalFile, err := processHTMLWithRoutes(rendered, page.SourcePath, routesDir, true, true)
	if err != nil {
		return "", fmt.Errorf("post-process HTML for %q: %w", page.SourcePath, err)
	}
	return finalFile, nil
}

// OptimiseImages adds responsive image attributes without rewriting links.
func OptimiseImages(documentText string) string {
	result, err := processHTML(documentText, "", false, true)
	if err != nil {
		return documentText
	}
	return result
}

func processHTML(documentText string, currentFilePath string, rewriteLinks bool, optimiseImages bool) (string, error) {
	return processHTMLWithRoutes(documentText, currentFilePath, "routes", rewriteLinks, optimiseImages)
}

func processHTMLWithRoutes(documentText string, currentFilePath string, routesDir string, rewriteLinks bool, optimiseImages bool) (string, error) {
	tokenizer := html.NewTokenizer(strings.NewReader(documentText))
	var output strings.Builder
	isFirstImage := true

	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			if errors.Is(tokenizer.Err(), io.EOF) {
				return output.String(), nil
			}
			return "", tokenizer.Err()
		}

		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			output.Write(tokenizer.Raw())
			continue
		}

		token := tokenizer.Token()
		changed := false
		if rewriteLinks {
			for attributeIndex := range token.Attr {
				attribute := &token.Attr[attributeIndex]
				if attribute.Key != "href" && attribute.Key != "src" {
					continue
				}
				if rewritten, ok := rewriteRelativeURLWithRoutes(attribute.Val, currentFilePath, routesDir); ok {
					attribute.Val = rewritten
					changed = true
				}
			}
		}

		if optimiseImages && token.Data == "img" {
			if optimiseImageToken(&token, isFirstImage) {
				changed = true
			}
			isFirstImage = false
		}

		if changed {
			output.WriteString(token.String())
		} else {
			output.Write(tokenizer.Raw())
		}
	}
}

func optimiseImageToken(token *html.Token, isFirst bool) bool {
	var source string
	for _, attribute := range token.Attr {
		if attribute.Key == "srcset" {
			return false
		}
		if attribute.Key == "src" {
			source = attribute.Val
		}
	}
	if source == "" {
		return false
	}

	srcset, ok := image.BuildSrcset(source)
	if !ok {
		return false
	}
	token.Attr = append(token.Attr, html.Attribute{Key: "srcset", Val: srcset})
	if !isFirst {
		if !hasAttribute(token.Attr, "loading") {
			token.Attr = append(token.Attr, html.Attribute{Key: "loading", Val: "lazy"})
		}
		if !hasAttribute(token.Attr, "decoding") {
			token.Attr = append(token.Attr, html.Attribute{Key: "decoding", Val: "async"})
		}
	}
	return true
}

func hasAttribute(attributes []html.Attribute, name string) bool {
	for _, attribute := range attributes {
		if attribute.Key == name {
			return true
		}
	}
	return false
}

func rewriteRelativeURL(rawURL string, currentFilePath string) (string, bool) {
	return rewriteRelativeURLWithRoutes(rawURL, currentFilePath, "routes")
}

func rewriteRelativeURLWithRoutes(rawURL string, currentFilePath string, routesDir string) (string, bool) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme != "" || parsedURL.Host != "" || parsedURL.Path == "" || strings.HasPrefix(rawURL, "//") || strings.HasPrefix(parsedURL.Path, "/") {
		return rawURL, false
	}

	targetPath := filepath.Join(filepath.Dir(currentFilePath), filepath.FromSlash(parsedURL.Path))
	targetPath = filepath.ToSlash(filepath.Clean(targetPath))
	fileType := strings.TrimPrefix(strings.ToLower(filepath.Ext(parsedURL.Path)), ".")
	finalPath, err := sitepath.OutputPath(routesDir, "build", targetPath, fileType)
	if err != nil {
		return rawURL, false
	}
	webPath := strings.TrimPrefix(finalPath, "build")
	webPath = strings.TrimSuffix(webPath, "index.html")
	webPath = "/" + strings.TrimPrefix(webPath, "/")

	parsedURL.Path = webPath
	parsedURL.RawPath = ""
	return parsedURL.String(), true
}

func extractText(documentText string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(documentText))
	var words []string
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return strings.Join(words, " ")
		case html.TextToken:
			words = append(words, strings.Fields(tokenizer.Token().Data)...)
		}
	}
}

func FindFinalPath(file types.File) string { //takes an original path, starting in 'routes' and resolves it to the location, ending in "build"
	path, err := sitepath.OutputPath("routes", "build", file.OriginalPath, file.Type)
	if err != nil {
		return ""
	}
	return path
}

func FindTemplate(path string, templates map[string]string) (template string, templatePath string) {
	parts := strings.Split(path, "/")
	// find the closest template to the file path by working upwards
	for i := len(parts) - 1; i > 0; i-- {
		pathToCheck := strings.Join(parts[0:i], "/") + "/"
		//fmt.Println("checking path", pathToCheck)
		template := templates[pathToCheck]
		if template != "" {
			return template, pathToCheck
		}
	}
	return "<!doctype html><body>{{slot}}</body>", ""
}
