// Package builder contains the shared production and development site build.
package builder

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/louis-bourgault/ssg/image"
	"github.com/louis-bourgault/ssg/index"
	"github.com/louis-bourgault/ssg/renderer"
	"github.com/louis-bourgault/ssg/types"
)

// Options configures a complete site build.
type Options struct {
	SourceDir string
	OutputDir string
}

// Build renders all source files into OutputDir. The output is assembled in a
// sibling staging directory and installed only after the complete build succeeds.
func Build(ctx context.Context, options Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if options.SourceDir == "" {
		return errors.New("source directory is required")
	}
	if options.OutputDir == "" {
		return errors.New("output directory is required")
	}

	sourceDir, err := filepath.Abs(options.SourceDir)
	if err != nil {
		return fmt.Errorf("resolve source directory: %w", err)
	}
	outputDir, err := filepath.Abs(options.OutputDir)
	if err != nil {
		return fmt.Errorf("resolve output directory: %w", err)
	}
	if sourceDir == outputDir || pathWithin(outputDir, sourceDir) {
		return errors.New("output directory must not contain the source directory")
	}

	files, templates, projectIndex, err := scanProject(ctx, sourceDir, outputDir)
	if err != nil {
		return err
	}

	outputParent := filepath.Dir(outputDir)
	if err := os.MkdirAll(outputParent, 0o755); err != nil {
		return fmt.Errorf("create output parent directory: %w", err)
	}
	stagingRoot, err := os.MkdirTemp(outputParent, "."+filepath.Base(outputDir)+"-staging-")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	defer os.RemoveAll(stagingRoot)

	renderOptions := renderer.Options{SourceDir: sourceDir, OutputDir: outputDir}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}

		relativeOutput, err := renderer.FinalRelativePath(file, sourceDir)
		if err != nil {
			return err
		}
		stagedLocation := filepath.Join(stagingRoot, relativeOutput)
		if !pathWithin(stagingRoot, stagedLocation) {
			return fmt.Errorf("output path %q escapes the staging directory", stagedLocation)
		}

		var finished []byte
		if strings.EqualFold(file.Type, "md") {
			template, _ := renderer.FindTemplate(file.OriginalPath, templates)
			content, err := os.ReadFile(file.OriginalPath)
			if err != nil {
				return fmt.Errorf("read Markdown file %q: %w", file.OriginalPath, err)
			}
			rendered, err := renderer.GenerateSingleFileWithOptions(string(content), template, file.OriginalPath, projectIndex, renderOptions)
			if err != nil {
				return err
			}
			finished = []byte(rendered)
		} else {
			finished, err = os.ReadFile(file.OriginalPath)
			if err != nil {
				return fmt.Errorf("read static file %q: %w", file.OriginalPath, err)
			}
			if image.IsSupportedRasterPath(file.OriginalPath) {
				// Image optimisation remains best-effort, as it was in the original
				// production builder. The original asset is always copied below.
				_ = image.GenerateImages(file.OriginalPath, stagedLocation)
			}
		}

		if err := os.MkdirAll(filepath.Dir(stagedLocation), 0o755); err != nil {
			return fmt.Errorf("create output directory %q: %w", filepath.Dir(stagedLocation), err)
		}
		if err := os.WriteFile(stagedLocation, finished, 0o644); err != nil {
			return fmt.Errorf("write output file %q: %w", stagedLocation, err)
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	return replaceDirectory(stagingRoot, outputDir)
}

func scanProject(ctx context.Context, sourceDir, outputDir string) ([]types.File, map[string]string, *index.ProjectIndex, error) {
	var files []types.File
	templates := make(map[string]string)
	projectIndex := index.NewProjectIndex()

	err := filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if path != sourceDir && pathWithin(outputDir, path) {
				return filepath.SkipDir
			}
			return nil
		}

		name := entry.Name()
		if name == "template.html" {
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read template %q: %w", path, err)
			}
			templates[filepath.Clean(filepath.Dir(path))+string(filepath.Separator)] = string(content)
			return nil
		}
		if name == ".index.json" {
			return nil
		}

		fileType := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
		file := types.File{OriginalPath: filepath.Clean(path), Type: fileType}
		files = append(files, file)
		if fileType == "md" {
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read Markdown file %q: %w", path, err)
			}
			if err := projectIndex.AddFile(file, string(content)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("scan source directory %q: %w", sourceDir, err)
	}
	return files, templates, projectIndex, nil
}

func replaceDirectory(stagingRoot, outputRoot string) error {
	parent := filepath.Dir(outputRoot)
	backupRoot, err := os.MkdirTemp(parent, "."+filepath.Base(outputRoot)+"-previous-")
	if err != nil {
		return fmt.Errorf("reserve previous-build path: %w", err)
	}
	if err := os.Remove(backupRoot); err != nil {
		return fmt.Errorf("prepare previous-build path: %w", err)
	}
	defer os.RemoveAll(backupRoot)

	hadPrevious := false
	if _, err := os.Lstat(outputRoot); err == nil {
		if err := os.Rename(outputRoot, backupRoot); err != nil {
			return fmt.Errorf("move previous build aside: %w", err)
		}
		hadPrevious = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect previous build: %w", err)
	}

	if err := os.Rename(stagingRoot, outputRoot); err != nil {
		if hadPrevious {
			if restoreErr := os.Rename(backupRoot, outputRoot); restoreErr != nil {
				return fmt.Errorf("install new build: %w (also failed to restore previous build: %v)", err, restoreErr)
			}
		}
		return fmt.Errorf("install new build: %w", err)
	}
	if hadPrevious {
		if err := os.RemoveAll(backupRoot); err != nil {
			return fmt.Errorf("remove previous build: %w", err)
		}
	}
	return nil
}

func pathWithin(parent, candidate string) bool {
	relative, err := filepath.Rel(parent, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
