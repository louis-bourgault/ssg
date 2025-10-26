package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "github.com/google/uuid"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type File struct {
	OriginalPath string
	Type         string
	FinalPath    string
}

func readFile(filename string) string {
	content, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	return string(content)
}

func main() {
	var rootPath string
	rootPath = "routes"
	traverseDirectory(rootPath)

}

func initDevServer() {
	//for testing purposes, for prod you would probably copy things to nginx or a dedicated file server
	dir := http.Dir("./build")
	fileServer := http.FileServer(dir)
	if err := http.ListenAndServe(":8080", fileServer); err != nil {
		panic(err)
	}
}

func traverseDirectory(rootPath string) {
	var filesFound = []File{}
	var templates = make(map[string]string)

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}
		if d.IsDir() {
			fmt.Println("directory:", path)
		} else {
			slashed := filepath.ToSlash(path)
			fmt.Println("file:", slashed)
			fileParts := strings.Split(slashed, "/")
			last := fileParts[len(fileParts)-1]
			directory := strings.TrimSuffix(slashed, last)
			dotSplit := strings.Split(last, ".")
			if last == "template.html" {
				templates[directory] = readFile(path)
				fmt.Println("added template to map for the path", directory)
			} else {
				filesFound = append(filesFound, File{
					OriginalPath: slashed,
					Type:         dotSplit[len(dotSplit)-1],
				})
			}

		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	for i := 0; i < len(filesFound); i++ {
		var finished []byte
		finalLocation := findFinalPath(filesFound[i])
		if filesFound[i].Type == "md" {

			template, path := findTemplate(filesFound[i].OriginalPath, templates)
			content := readFile(filepath.FromSlash(filesFound[i].OriginalPath))
			fmt.Println("Generating with the template ", path, "and the file", filesFound[i].OriginalPath)
			finished = []byte(fixLinksAndImages(generateSingleFile(content, template), filesFound[i].OriginalPath))
		} else {
			finished = []byte(readFile(filepath.FromSlash(filesFound[i].OriginalPath)))
		}
		dirPath := filepath.FromSlash(filepath.Dir(finalLocation))
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			fmt.Printf("Error creating directory: %v\n", err)
			return
		}
		err = os.WriteFile(filepath.FromSlash(finalLocation), finished, 0644)
		if err != nil {
			panic(err)
		}

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
	targetFile := File{
		OriginalPath: routesPath,
		Type:         fileType,
	}

	finalPath := findFinalPath(targetFile)
	webPath, _ := strings.CutPrefix(finalPath, "build")
	webPath = strings.TrimSuffix(webPath, "index.html")
	if !strings.HasPrefix(webPath, "/") {
		webPath = "/" + webPath
	}
	//fix double slashes if we have any
	webPath = strings.ReplaceAll(webPath, "//", "/")

	return webPath
}

func findFinalPath(file File) string { //takes an original path, starting in 'routes' and resolves it to the location, ending in "build"
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

func findTemplate(path string, templates map[string]string) (template string, templatePath string) {
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

func generateSingleFile(content string, template string) string {
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

	finalFile := strings.Join([]string{populateMeta(context, templateParts[0]), buf.String(), populateMeta(context, templateParts[1])}, "")
	return finalFile
}

func populateMeta(ctx parser.Context, documentText string) string {
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
