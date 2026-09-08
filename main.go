package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/louis-bourgault/ssg/dev"
	"github.com/louis-bourgault/ssg/image"
	"github.com/louis-bourgault/ssg/index"
	"github.com/louis-bourgault/ssg/renderer"
	"github.com/louis-bourgault/ssg/sitepath"
	"github.com/louis-bourgault/ssg/templating"
	"github.com/louis-bourgault/ssg/types"
)

type templateSource struct {
	path    string
	content string
}

func readFile(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	var rootPath string
	rootPath = "routes"

	if len(os.Args) < 2 {
		if err := BuildFromDirectory(rootPath); err != nil {
			return err
		}
		fmt.Println("Build Completed")
	} else {
		subcommand := os.Args[1]
		switch subcommand {
		case "dev":
			fmt.Println("Running Development Server")
			return dev.RunDevServer()
		case "serve":
			//run simple static file server for testing a built site
			if err := BuildFromDirectory(rootPath); err != nil {
				return err
			}
			fmt.Println("Build Completed")
			fmt.Println("Running Static File Server on port 8080")
			fileServer := http.FileServer(http.Dir("./build"))
			server := &http.Server{Addr: ":8080", Handler: fileServer}
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("serve build directory: %w", err)
			}
		default:
			return fmt.Errorf("unknown command %q; run without a command to build, or use 'dev' or 'serve'", subcommand)
		}
	}

	return nil
}

func BuildFromDirectory(rootPath string) error {
	var filesFound = []types.File{}
	var templates = make(map[string]templateSource)
	projectIndices := index.NewProjectIndexForRoutes(rootPath)

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			fmt.Println("directory:", path)
			projectIndices.AddDirectory(path)
		} else {
			slashed := filepath.ToSlash(path)
			fmt.Println("file:", slashed)
			fileParts := strings.Split(slashed, "/")
			last := fileParts[len(fileParts)-1]
			directory := strings.TrimSuffix(slashed, last)
			dotSplit := strings.Split(last, ".")
			if last == "template.html" {
				template, err := readFile(path)
				if err != nil {
					return fmt.Errorf("read template %q: %w", path, err)
				}
				templates[directory] = templateSource{path: slashed, content: template}
				fmt.Println("added template to map for the path", directory)
			} else if last != ".index.json" {
				filesFound = append(filesFound, types.File{
					OriginalPath: slashed,
					Type:         dotSplit[len(dotSplit)-1],
				})
			}

		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan source directory %q: %w", rootPath, err)
	}
	applicableTemplates := make(map[string]templateSource)
	for i := range filesFound {
		if filesFound[i].Type == "md" {
			applicableTemplates[filesFound[i].OriginalPath] = findTemplateSource(filesFound[i].OriginalPath, templates)
		}
	}

	for i := 0; i < len(filesFound); i++ {
		if filesFound[i].Type == "md" { //only index markdown files

			fileContent, err := readFile(filepath.FromSlash(filesFound[i].OriginalPath))
			if err != nil {
				return fmt.Errorf("read Markdown file %q: %w", filesFound[i].OriginalPath, err)
			}
			if err := projectIndices.AddFile(filesFound[i], fileContent); err != nil {
				return err
			}
		}
	}
	templateCache := templating.NewCache()
	templatePaths := make([]string, 0, len(templates))
	templatesByPath := make(map[string]templateSource, len(templates))
	for _, template := range templates {
		templatePaths = append(templatePaths, template.path)
		templatesByPath[template.path] = template
	}
	sort.Strings(templatePaths)
	for _, templatePath := range templatePaths {
		template := templatesByPath[templatePath]
		if _, err := templateCache.Compile(template.path, template.content); err != nil {
			return err
		}
	}

	stagingRoot, err := os.MkdirTemp(".", ".ssg-build-")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	defer os.RemoveAll(stagingRoot)

	for i := 0; i < len(filesFound); i++ {
		var finished []byte
		finalLocation, err := sitepath.OutputPath(rootPath, "build", filesFound[i].OriginalPath, filesFound[i].Type)
		if err != nil {
			return err
		}
		stagedLocation, err := stagedBuildPath(stagingRoot, finalLocation)
		if err != nil {
			return err
		}

		if filesFound[i].Type == "md" {
			template := applicableTemplates[filesFound[i].OriginalPath]
			compiled, err := templateCache.Compile(template.path, template.content)
			if err != nil {
				return err
			}
			page, exists := projectIndices.Page(filesFound[i].OriginalPath)
			if !exists {
				return fmt.Errorf("indexed Markdown page %q is unavailable", filesFound[i].OriginalPath)
			}
			fmt.Println("Generating with the template ", template.path, "and the file", filesFound[i].OriginalPath)
			rendered, err := renderer.GeneratePage(*page, compiled, projectIndices)
			if err != nil {
				return err
			}
			finished = []byte(rendered)
		} else { //this would be the place to put webp logic
			file, err := os.ReadFile(filepath.FromSlash(filesFound[i].OriginalPath))
			if err != nil {
				return fmt.Errorf("read static file %q: %w", filesFound[i].OriginalPath, err)
			}
			finished = file
			if image.IsSupportedRasterPath(filesFound[i].OriginalPath) {
				if err := image.GenerateImages(filesFound[i].OriginalPath, stagedLocation); err != nil {
					fmt.Printf("Could not optimise image %s: %v\n", filesFound[i].OriginalPath, err)
				}
			}

		}
		dirPath := filepath.Dir(stagedLocation)
		err = os.MkdirAll(dirPath, 0755)
		if err != nil {
			return fmt.Errorf("create output directory %q: %w", dirPath, err)
		}
		err = os.WriteFile(stagedLocation, finished, 0644)
		if err != nil {
			return fmt.Errorf("write output file %q: %w", stagedLocation, err)
		}

	}

	if err := replaceBuildDirectory(stagingRoot, "build"); err != nil {
		return err
	}
	return nil
}

func findTemplateSource(path string, templates map[string]templateSource) templateSource {
	path = filepath.ToSlash(filepath.Clean(path))
	for directory := filepath.ToSlash(filepath.Dir(path)); directory != "."; {
		key := strings.TrimSuffix(directory, "/") + "/"
		if template, exists := templates[key]; exists {
			return template
		}
		parent := filepath.ToSlash(filepath.Dir(directory))
		if parent == directory {
			break
		}
		directory = parent
	}
	return templateSource{path: "<default template>", content: "<!doctype html><body>{{slot}}</body>"}
}

func stagedBuildPath(stagingRoot string, finalLocation string) (string, error) {
	relativePath, err := filepath.Rel("build", filepath.FromSlash(finalLocation))
	if err != nil {
		return "", fmt.Errorf("resolve output path %q: %w", finalLocation, err)
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("output path %q escapes the build directory", finalLocation)
	}
	return filepath.Join(stagingRoot, relativePath), nil
}

func replaceBuildDirectory(stagingRoot string, outputRoot string) error {
	backupRoot, err := os.MkdirTemp(".", ".ssg-previous-build-")
	if err != nil {
		return fmt.Errorf("reserve previous-build path: %w", err)
	}
	if err := os.Remove(backupRoot); err != nil {
		return fmt.Errorf("prepare previous-build path: %w", err)
	}

	hadPreviousBuild := false
	if _, err := os.Lstat(outputRoot); err == nil {
		if err := os.Rename(outputRoot, backupRoot); err != nil {
			return fmt.Errorf("move previous build aside: %w", err)
		}
		hadPreviousBuild = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect previous build: %w", err)
	}

	if err := os.Rename(stagingRoot, outputRoot); err != nil {
		if hadPreviousBuild {
			if restoreErr := os.Rename(backupRoot, outputRoot); restoreErr != nil {
				return fmt.Errorf("install new build: %w (also failed to restore previous build: %v)", err, restoreErr)
			}
		}
		return fmt.Errorf("install new build: %w", err)
	}

	if hadPreviousBuild {
		if err := os.RemoveAll(backupRoot); err != nil {
			return fmt.Errorf("remove previous build: %w", err)
		}
	}
	return nil
}
