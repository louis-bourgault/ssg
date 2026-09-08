package image

import (
	stdimage "image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateImagesCreatesNestedOutputDirectory(t *testing.T) {
	tempDir := t.TempDir()
	originalPath := filepath.Join(tempDir, "source.png")
	outputPath := filepath.Join(tempDir, "build", "nested", "photo.png")

	writeTestPNG(t, originalPath, 8, 4)

	if err := GenerateImages(originalPath, outputPath); err != nil {
		t.Fatalf("GenerateImages returned an error: %v", err)
	}

	generatedPath := filepath.Join(tempDir, "build", "nested", "photo.8.webp")
	if _, err := os.Stat(generatedPath); err != nil {
		t.Fatalf("generated image was not created: %v", err)
	}
}

func TestGenerateImagesRejectsUnsupportedType(t *testing.T) {
	err := GenerateImages("picture.svg", filepath.Join(t.TempDir(), "picture.svg"))
	if err == nil {
		t.Fatal("GenerateImages should reject SVG input")
	}
}

func TestBuildSrcsetRejectsUnsupportedSources(t *testing.T) {
	tests := []string{
		"/icon.svg",
		"https://example.com/photo.png",
		"//example.com/photo.png",
		"data:image/png;base64,abc",
		"/missing.png",
	}

	for _, source := range tests {
		if srcset, ok := BuildSrcset(source); ok {
			t.Errorf("BuildSrcset accepted unsupported source %q: %s", source, srcset)
		}
	}
}

func TestBuildSrcsetForLocalRasterImage(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	writeTestPNG(t, filepath.Join(tempDir, "routes", "photo.png"), 8, 4)

	srcset, ok := BuildSrcset("/photo.png")
	if !ok {
		t.Fatal("BuildSrcset rejected a local raster image")
	}
	if !strings.Contains(srcset, "/photo.8.webp 8w") {
		t.Fatalf("BuildSrcset did not return the expected candidate: %s", srcset)
	}
}

func writeTestPNG(t *testing.T, filePath string, width int, height int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("create test image directory: %v", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("create test image: %v", err)
	}

	img := stdimage.NewRGBA(stdimage.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}

	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatalf("encode test image: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close test image: %v", err)
	}
}
