package renderer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/louis-bourgault/ssg/image"
	"github.com/louis-bourgault/ssg/index"
	"github.com/louis-bourgault/ssg/types"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkHTML "github.com/yuin/goldmark/renderer/html"
	"golang.org/x/net/html"
)

var (
	metaPattern = regexp.MustCompile(`{{meta\.([^}]+)}}`)
	eachPattern = regexp.MustCompile(`(?s){{#each\s+([^}]+)}}(.*?){{/each}}`)
	itemPattern = regexp.MustCompile(`{{item\.([^}]+)}}`)
)

func GenerateSingleFile(content string, template string, path string, index *index.ProjectIndex) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			goldmarkHTML.WithHardWraps(),
			goldmarkHTML.WithXHTML(),
			goldmarkHTML.WithUnsafe(),
		),
	)
	fmt.Println("Generating single file for path:", path)
	var buf bytes.Buffer
	context := parser.NewContext()
	if err := md.Convert([]byte(content), &buf, parser.WithContext(context)); err != nil {
		return "", fmt.Errorf("render Markdown %q: %w", path, err)
	}
	templateBefore, templateAfter, found := strings.Cut(template, "{{slot}}")
	if !found {
		return "", fmt.Errorf("template for %q must contain {{slot}}", path)
	}
	if strings.Contains(templateAfter, "{{slot}}") {
		return "", fmt.Errorf("template for %q must contain exactly one {{slot}}", path)
	}
	joined := strings.Join([]string{PopulateMeta(context, templateBefore), buf.String(), PopulateMeta(context, templateAfter)}, "")
	eachPop, err := populateEach(joined, index, path)
	if err != nil {
		return "", err
	}
	finalFile, err := processHTML(eachPop, path, true, true)
	if err != nil {
		return "", fmt.Errorf("post-process HTML for %q: %w", path, err)
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

func PopulateMeta(ctx parser.Context, documentText string) string {
	meta := meta.Get(ctx)

	result := metaPattern.ReplaceAllStringFunc(documentText, func(match string) string {
		key := metaPattern.FindStringSubmatch(match)[1]
		value := meta[key]
		return fmt.Sprintf("%v", value)
	})

	return result
}

func populateEach(documentText string, index *index.ProjectIndex, path string) (string, error) { //at the moment only works for directories, but i would like other types of collections such as headings in the document
	//fmt.Println("populating each in the document")
	//detect any area that starts with {{#each [...]}} and ends with {{/each}}
	var renderErr error
	content := eachPattern.ReplaceAllStringFunc(documentText, func(match string) string {
		if renderErr != nil {
			return match
		}
		//fmt.Println("processing each block:", match)
		eachInner := eachPattern.FindStringSubmatch(match)
		fieldName := eachInner[1]
		blockContent := eachInner[2]
		//fmt.Println("field name:", fieldName)
		//fmt.Println("block content:", blockContent)

		whereFrom := strings.Split(fieldName, " ")[0]
		//fmt.Println("where from:", whereFrom)

		folderToLook := filepath.Join(filepath.Dir(path), whereFrom)
		//fmt.Println("folder to look:", folderToLook)

		if index == nil {
			renderErr = fmt.Errorf("render collection in %q: project index is unavailable", path)
			return match
		}
		directoryIndex, exists := index.Directories[folderToLook]
		if !exists {
			renderErr = fmt.Errorf("render collection in %q: directory %q is not indexed", path, folderToLook)
			return match
		}
		//fmt.Println("found directory index for", folderToLook)

		compiledHTML := ""

		// Regex to find all {{item.PropertyName}} patterns in the block content
		for _, fileIndex := range directoryIndex.Files {
			itemContent := itemPattern.ReplaceAllStringFunc(blockContent, func(itemMatch string) string {
				if renderErr != nil {
					return itemMatch
				}
				//fmt.Println("item match", itemMatch)
				propertyName := itemPattern.FindStringSubmatch(itemMatch)[1]
				if strings.HasPrefix(propertyName, "_preview") {
					previewLengthStr := strings.TrimPrefix(propertyName, "_preview")
					previewLength, err := strconv.Atoi(previewLengthStr)
					if err != nil || previewLength < 0 {
						renderErr = fmt.Errorf("invalid preview length %q in %q", previewLengthStr, path)
						return itemMatch
					}
					//there could be a better way than converting to html and then removing, but this does the job for now
					originalFilePath := fileIndex.File.OriginalPath
					fileContentBytes, err := os.ReadFile(originalFilePath)
					if err != nil {
						renderErr = fmt.Errorf("read preview source %q: %w", originalFilePath, err)
						return itemMatch
					}
					fileContent := stripYamlProperties(string(fileContentBytes))
					md := goldmark.New()
					var buf bytes.Buffer
					if err := md.Convert([]byte(fileContent), &buf); err != nil {
						renderErr = fmt.Errorf("render preview source %q: %w", originalFilePath, err)
						return itemMatch
					}

					plaintext := extractText(buf.String())

					previewRunes := []rune(plaintext)
					if len(previewRunes) > previewLength {
						return string(previewRunes[:previewLength]) + "..."
					}
					return plaintext
				}
				//fmt.Println("property name:", propertyName)
				value, exists := fileIndex.Properties[propertyName]
				if !exists {
					return ""
				}
				return fmt.Sprintf("%v", value)
			})
			compiledHTML += itemContent
		}

		return compiledHTML
	})

	return content, renderErr
}

func stripYamlProperties(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}

	lines := strings.Split(content, "\n")
	endIndex := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIndex = i
			break
		}
	}

	if endIndex == -1 {
		return content
	}

	return strings.Join(lines[endIndex+1:], "\n")
}

func processHTML(documentText string, currentFilePath string, rewriteLinks bool, optimiseImages bool) (string, error) {
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
				if rewritten, ok := rewriteRelativeURL(attribute.Val, currentFilePath); ok {
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
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme != "" || parsedURL.Host != "" || parsedURL.Path == "" || strings.HasPrefix(rawURL, "//") || strings.HasPrefix(parsedURL.Path, "/") {
		return rawURL, false
	}

	targetPath := filepath.Join(filepath.Dir(currentFilePath), filepath.FromSlash(parsedURL.Path))
	targetPath = filepath.ToSlash(filepath.Clean(targetPath))
	fileType := strings.TrimPrefix(strings.ToLower(filepath.Ext(parsedURL.Path)), ".")
	targetFile := types.File{OriginalPath: targetPath, Type: fileType}

	finalPath := FindFinalPath(targetFile)
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
	trimmed, _ := strings.CutPrefix(file.OriginalPath, "routes")
	before, mdFound := strings.CutSuffix(trimmed, "index.md")
	if mdFound == true {

		return strings.Join([]string{"build", before, "index.html"}, "")
	}
	before, htmlFound := strings.CutSuffix(trimmed, "index.html")
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
		//fmt.Println("checking path", pathToCheck)
		template := templates[pathToCheck]
		if template != "" {
			return template, pathToCheck
		}
	}
	return "<!doctype html><body>{{slot}}</body>", ""
}
