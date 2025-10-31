package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/google/uuid"
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
		buildFromDirectory(rootPath)
		fmt.Println("Build Completed")
	} else {
		subcommand := os.Args[1]
		switch subcommand {
		case "dev":
			fmt.Println("Running Development Server")
			RunDevServer()
		default:
			fmt.Println("Unknown command. Either run without a command for build, or use the command 'dev' for the development server")
		}
	}

}

func RunDevServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//pathSegments := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")

		//What we need to do here:
		// - Go grab the file for the route. If it ends in a file name, we just grab that (for example image assets), but if it ends in a slash it could either be an index.md file, a index.html file, or a file in the previous directory
		fmt.Println("Serving URL path", r.URL.Path)
		if !strings.HasSuffix(r.URL.Path, "/") {
			//this is an image
			content, err := readFile("routes" + filepath.FromSlash(r.URL.Path))
			if err != nil {
				panic(err)
			}
			w.Write([]byte(content))
		} else {
			//check for index.html in the previous directory
			var file string
			toCheck := filepath.Join("routes"+filepath.FromSlash(r.URL.Path), "..", "index.html")
			file, err := readFile(toCheck)
			if os.IsNotExist(err) {
				toCheck := filepath.Join("routes"+filepath.FromSlash(r.URL.Path), "..", "index.html")
				file, err = readFile(toCheck)
				if os.IsNotExist(err) {
					//if neither of these files exist, we can throw a 404
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprintf(w, "404 Not Found: The requested resource '%s' could not be found.", r.URL.Path)
				}

			}
			fmt.Println(file)

		}

	})

	fmt.Println("Server starting on port 8080...")
	http.ListenAndServe(":8080", nil)
}

func FindTemplateRuntime(contentLocation string) (templateLocation string) {
	dir := filepath.Dir(contentLocation)
	for {
		templatePath := filepath.Join(dir, "template.html")
		if _, err := os.Stat(templatePath); err == nil {
			return templatePath
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func createTemplateMap(rootPath string) map[string]string {
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
			if last == "template.html" {
				templates[directory], _ = readFile(path)
				fmt.Println("added template to map for the path", directory)
			}

		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return templates
}

func buildFromDirectory(rootPath string) {
	var filesFound = []types.File{}
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
		var finished []byte
		finalLocation := renderer.FindFinalPath(filesFound[i])
		if filesFound[i].Type == "md" {
			template, path := renderer.FindTemplate(filesFound[i].OriginalPath, templates)
			content, _ := readFile(filepath.FromSlash(filesFound[i].OriginalPath))
			fmt.Println("Generating with the template ", path, "and the file", filesFound[i].OriginalPath)
			finished = []byte(renderer.GenerateSingleFile(content, template, filesFound[i]))
		} else {
			file, _ := readFile(filepath.FromSlash(filesFound[i].OriginalPath))
			finished = []byte(file)
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
