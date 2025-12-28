package renderer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/louis-bourgault/ssg/index"
	"github.com/louis-bourgault/ssg/types"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

func GenerateSingleFile(content string, template string, path string, index *index.ProjectIndex) string {
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
	fmt.Println("Generating single file for path:", path)
	var buf bytes.Buffer
	context := parser.NewContext()
	if err := md.Convert([]byte(content), &buf, parser.WithContext(context)); err != nil {
		panic(err)
	}
	templateParts := strings.Split(template, "{{slot}}")
	joined := strings.Join([]string{PopulateMeta(context, templateParts[0]), buf.String(), PopulateMeta(context, templateParts[1])}, "")
	eachPop := populateEach(joined, index, path)
	finalFile := fixLinksAndImages(eachPop, path)

	return finalFile
}

func PopulateMeta(ctx parser.Context, documentText string) string {
	meta := meta.Get(ctx)

	metaPattern := regexp.MustCompile(`{{meta\.([^}]+)}}`)
	result := metaPattern.ReplaceAllStringFunc(documentText, func(match string) string {
		key := metaPattern.FindStringSubmatch(match)[1]
		value := meta[key]
		return fmt.Sprintf("%v", value)
	})

	return result
}

func populateEach(documentText string, index *index.ProjectIndex, path string) string {
	//fmt.Println("populating each in the document")
	//detect any area that starts with {{#each [...]}} and ends with {{/each}}
	eachPattern := regexp.MustCompile(`(?s){{#each\s+([^}]+)}}(.*?){{/each}}`)
	content := eachPattern.ReplaceAllStringFunc(documentText, func(match string) string {
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

		directoryIndex, exists := index.Directories[folderToLook]
		if !exists {
			fmt.Println("no directory index found for", folderToLook)
			return ""
		}
		//fmt.Println("found directory index for", folderToLook)

		compiledHTML := ""

		// Regex to find all {{item.PropertyName}} patterns in the block content
		itemPattern := regexp.MustCompile(`{{item\.([^}]+)}}`)

		for _, fileIndex := range directoryIndex.Files {
			itemContent := itemPattern.ReplaceAllStringFunc(blockContent, func(itemMatch string) string {
				//fmt.Println("item match", itemMatch)
				propertyName := itemPattern.FindStringSubmatch(itemMatch)[1]
				if strings.HasPrefix(propertyName, "_preview") {
					previewLengthStr := strings.TrimPrefix(propertyName, "_preview")
					previewLength, err := strconv.Atoi(previewLengthStr)
					if err != nil {
						fmt.Println("Error parsing preview length:", err)
						return ""
					}
					//there could be a better way than converting to html and then removing, but this does the job for now
					originalFilePath := fileIndex.File.OriginalPath
					fileContentBytes, err := os.ReadFile(originalFilePath)
					if err != nil {
						fmt.Println("Error reading file for preview:", err)
						return ""
					}
					fileContent := stripYamlProperties(string(fileContentBytes))
					md := goldmark.New()
					var buf bytes.Buffer
					if err := md.Convert([]byte(fileContent), &buf); err != nil {
						fmt.Println("Error converting markdown:", err)
						return ""
					}

					//get rid of html tags
					htmlTagPattern := regexp.MustCompile(`<[^>]*>`)
					plaintext := htmlTagPattern.ReplaceAllString(buf.String(), "")
					plaintext = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(plaintext, " "))

					if len(plaintext) > previewLength {
						return plaintext[:previewLength] + "..."
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

	return content
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
	//fmt.Println("resolving the link", relativeUrl, "coming from", currentFilePath)

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
		//fmt.Println("checking path", pathToCheck)
		template := templates[pathToCheck]
		if template != "" {
			return template, pathToCheck
		}
	}
	return "<!doctype html><body>{{slot}}</body>", ""
}
