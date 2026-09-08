package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/google/uuid"
	"github.com/louis-bourgault/ssg/dev"
	"github.com/louis-bourgault/ssg/image"
	"github.com/louis-bourgault/ssg/index"
	"github.com/louis-bourgault/ssg/renderer"
	"github.com/louis-bourgault/ssg/types"
)

func readFile(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	return string(content), nil
}

func main() {
	var rootPath string
	rootPath = "routes"

	if len(os.Args) < 2 {
		BuildFromDirectory(rootPath)
		fmt.Println("Build Completed")
	} else {
		subcommand := os.Args[1]
		switch subcommand {
		case "dev":
			fmt.Println("Running Development Server")
			dev.RunDevServer()
		case "serve":
			//run simple static file server for testing a built site
			BuildFromDirectory(rootPath)
			fmt.Println("Build Completed")
			fmt.Println("Running Static File Server on port 8080")
			fs := http.FileServer(http.Dir("./build"))
			http.Handle("/", fs)
			http.ListenAndServe(":8080", nil)
		default:
			fmt.Println("Unknown command. Either run without a command for build, or use the command 'dev' for the development server")
		}
	}

}

func BuildFromDirectory(rootPath string) {
	var filesFound = []types.File{}
	var templates = make(map[string]string)
	var projectIndices = index.ProjectIndex{
		Directories: map[string]*index.DirectoryIndex{},
	}

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
				templates[directory], _ = readFile(path)
				fmt.Println("added template to map for the path", directory)
			} else {
				filesFound = append(filesFound, types.File{
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
		if filesFound[i].Type == "md" { //only index markdown files

			fileContent, err := readFile(filepath.FromSlash(filesFound[i].OriginalPath))
			if err != nil {
				panic(err)
			}
			projectIndices.AddFile(filesFound[i], fileContent)
		}
	}
	projectIndices.Finalise()

	for i := 0; i < len(filesFound); i++ {
		var finished []byte
		finalLocation := renderer.FindFinalPath(filesFound[i])

		if filesFound[i].Type == "md" {
			template, path := renderer.FindTemplate(filesFound[i].OriginalPath, templates)
			content, _ := readFile(filepath.FromSlash(filesFound[i].OriginalPath))
			fmt.Println("Generating with the template ", path, "and the file", filesFound[i].OriginalPath)
			finished = []byte(renderer.GenerateSingleFile(content, template, filesFound[i].OriginalPath, &projectIndices))
		} else { //this would be the place to put webp logic
			file, _ := readFile(filepath.FromSlash(filesFound[i].OriginalPath))
			finished = []byte(file)
			if image.IsSupportedRasterPath(filesFound[i].OriginalPath) {
				if err := image.GenerateImages(filesFound[i].OriginalPath, finalLocation); err != nil {
					fmt.Printf("Could not optimise image %s: %v\n", filesFound[i].OriginalPath, err)
				}
			}

		}
		dirPath := filepath.FromSlash(filepath.Dir(finalLocation))
		err = os.MkdirAll(dirPath, 0755)
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
