package renderer

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/louis-bourgault/ssg/types"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

func GenerateSingleFile(content string, template string, metadata types.File) string {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)
	var buf bytes.Buffer
	context := parser.NewContext()
	if err := md.Convert([]byte(content), &buf, parser.WithContext(context)); err != nil {
		panic(err)
	}
	templateParts := strings.Split(template, "{{slot}}")

	finalFile := fixLinksAndImages(strings.Join([]string{PopulateMeta(context, templateParts[0]), buf.String(), PopulateMeta(context, templateParts[1])}, ""), metadata.OriginalPath)
	return finalFile
}

func PopulateMeta(ctx parser.Context, documentText string) string {
	meta := meta.Get(ctx)

	parts := strings.Split(documentText, "{{meta.")
	if len(parts) == 1 {
		return documentText
	} else {
		text := parts[0]
		for i := 1; i < len(parts); i++ { //we ignore the first one since
			split := strings.Split(parts[i], "}}")
			key := split[0]
			value := meta[key]
			asString := fmt.Sprintf("%v", value)
			if len(split) == 2 { //there is more stuff afterwards
				text = strings.Join([]string{text, asString, split[1]}, "")
			} else {
				text = strings.Join([]string{text, asString}, "")
			}
		}
		return text
	}
}

func fixLinksAndImages(htmlContent string, currentFilePath string) string {
	hrefPattern := regexp.MustCompile(`href="([^"]*)"`)
	srcPattern := regexp.MustCompile(`src="([^"]*)"`)

	htmlContent = hrefPattern.ReplaceAllStringFunc(htmlContent, func(match string) string {
		url := hrefPattern.FindStringSubmatch(match)[1]
		if isRelativeFileLink(url) {
			newUrl := resolveRelativeLink(url, currentFilePath)
			return fmt.Sprintf(`href="%s"`, newUrl)
		}
		return match
	})
	htmlContent = srcPattern.ReplaceAllStringFunc(htmlContent, func(match string) string {
		url := srcPattern.FindStringSubmatch(match)[1]
		if isRelativeFileLink(url) {
			newUrl := resolveRelativeLink(url, currentFilePath)
			return fmt.Sprintf(`src="%s"`, newUrl)
		}
		return match
	})

	return htmlContent
}

func isRelativeFileLink(url string) bool {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "//") {
		return false
	}
	if strings.HasPrefix(url, "/") {
		return false
	}
	if strings.HasPrefix(url, "#") || strings.HasPrefix(url, "mailto:") || strings.HasPrefix(url, "tel:") {
		return false
	}
	return true
}

func resolveRelativeLink(relativeUrl string, currentFilePath string) string { //TODO: write this myself
	fmt.Println("resolving the link", relativeUrl, "coming from", currentFilePath)

	//directory of current file
	currentDir := filepath.Dir(currentFilePath)

	// filepath.Join handles .. and . automatically
	targetPath := filepath.Join(currentDir, relativeUrl)
	targetPath = filepath.Clean(targetPath)
	targetPath = filepath.ToSlash(targetPath)

	routesPath, _ := strings.CutPrefix(targetPath, "build")

	parts := strings.Split(routesPath, ".")
	var fileType string
	if len(parts) > 1 {
		fileType = parts[len(parts)-1]
	} else {
		fileType = ""
	}
	targetFile := types.File{
		OriginalPath: routesPath,
		Type:         fileType,
	}

	finalPath := FindFinalPath(targetFile)
	webPath, _ := strings.CutPrefix(finalPath, "build")
	webPath = strings.TrimSuffix(webPath, "index.html")
	if !strings.HasPrefix(webPath, "/") {
		webPath = "/" + webPath
	}
	//fix double slashes if we have any
	webPath = strings.ReplaceAll(webPath, "//", "/")

	return webPath
}

func FindFinalPath(file types.File) string { //takes an original path, starting in 'routes' and resolves it to the location, ending in "build"
	trimmed, _ := strings.CutPrefix(file.OriginalPath, "routes")
	before, mdFound := strings.CutSuffix(trimmed, "index.md")
	if mdFound == true {

		return strings.Join([]string{"build", before, "index.html"}, "")
	}
	before, htmlFound := strings.CutSuffix(trimmed, "index.md")
	if htmlFound == true {
		return strings.Join([]string{"build", before, "index.html"}, "")
	}
	if file.Type == "md" {
		// /routes/about.md => /routes/about/index.html
		before, _ := strings.CutSuffix(trimmed, ".md")
		return strings.Join([]string{"build", before, "/index.html"}, "")
	}
	return strings.Join([]string{"build", trimmed}, "") //for static images, assets, etc, let's leave them where they are for now

}

func FindTemplate(path string, templates map[string]string) (template string, templatePath string) {
	parts := strings.Split(path, "/")
	// find the closest template to the file path by working upwards
	for i := len(parts) - 1; i > 0; i-- {
		pathToCheck := strings.Join(parts[0:i], "/") + "/"
		fmt.Println("checking path", pathToCheck)
		template := templates[pathToCheck]
		if template != "" {
			return template, pathToCheck
		}
	}
	return "<!doctype html><body>{{slot}}</body>", ""
}
