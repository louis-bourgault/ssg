package image

import (
	"fmt"
	"image"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "github.com/kolesa-team/go-webp/decoder"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
	"github.com/louis-bourgault/ssg/types"
	"github.com/nfnt/resize"
)

func IsSupportedRasterPath(filePath string) bool {
	extension := strings.ToLower(filepath.Ext(filePath))
	switch extension {
	case ".gif", ".jpeg", ".jpg", ".png", ".webp":
		return true
	default:
		return false
	}
}

func FindFinalDimensions(ow int, oh int) []types.ImageSize { //ow, oh mean original width and height. This function returns a list of the dimensions that this will be transformed into.
	if ow <= 0 || oh <= 0 {
		return nil
	}

	imageSizes := []types.ImageSize{}
	aspect := float64(oh) / float64(ow)

	widths := []int{0, 1920, 1200, 650}
	//we go down and find applicable image sizes based on certain stops.
	//By default, these will be pegged to a set of values for the longest side, and it will preserve the aspect ratio. However, in future, it may be possible to customise these preset values.
	// For example, for a 2000x3000 photo, it may be 1920x1280,
	// a 0 value in the array means to include the original resolution
	for i := 0; i < len(widths); i++ {
		if widths[i] == 0 {
			imageSizes = append(imageSizes, types.ImageSize{W: ow, H: oh})
		} else {
			if ow > oh {
				//width longest
				if ow > widths[i] {
					newWidth := widths[i]
					newHeight := int(float64(newWidth) * aspect)
					imageSizes = append(imageSizes, types.ImageSize{W: newWidth, H: newHeight})
				}
			} else {
				//height longest
				if oh > widths[i] {
					newHeight := widths[i]
					newWidth := int(float64(newHeight) / aspect)
					imageSizes = append(imageSizes, types.ImageSize{W: newWidth, H: newHeight})
				}
			}
		}
	}

	return imageSizes
}

func GenerateImages(originalPath string, finalPath string) error {
	if !IsSupportedRasterPath(originalPath) {
		return fmt.Errorf("unsupported image type %q", filepath.Ext(originalPath))
	}

	file, err := os.Open(originalPath)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	dimensionsToGenerate := FindFinalDimensions(width, height)
	if len(dimensionsToGenerate) == 0 {
		return fmt.Errorf("image has invalid dimensions %dx%d", width, height)
	}

	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return fmt.Errorf("create image output directory: %w", err)
	}

	options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 75)
	if err != nil {
		return fmt.Errorf("create WebP encoder: %w", err)
	}

	for _, size := range dimensionsToGenerate {
		resized := resize.Resize(uint(size.W), uint(size.H), img, resize.Lanczos3)
		ext := filepath.Ext(finalPath)
		baseFileName := finalPath[:len(finalPath)-len(ext)]
		locationToSave := baseFileName + "." + strconv.Itoa(size.W) + ".webp"
		outputFile, err := os.Create(locationToSave)
		if err != nil {
			return fmt.Errorf("create generated image %q: %w", locationToSave, err)
		}
		if err := webp.Encode(outputFile, resized, options); err != nil {
			outputFile.Close()
			return fmt.Errorf("encode generated image %q: %w", locationToSave, err)
		}
		if err := outputFile.Close(); err != nil {
			return fmt.Errorf("close generated image %q: %w", locationToSave, err)
		}
	}

	return nil
}

func BuildSrcset(source string) (string, bool) {
	return BuildSrcsetFrom(source, "routes")
}

// BuildSrcsetFrom builds a responsive srcset using assets below sourceDir.
func BuildSrcsetFrom(source string, sourceDir string) (string, bool) {
	//TODO: This function is really slow for what it does, which is a risk for things like the Cloudflare 20 minute site build. A single run for a large image often takes multiple seconds.
	// It may be better to go through the directory and pattern match the name (since they're already generated) instead of deterministically figuring out what resolutions it would be from scratch

	srcURL, err := url.Parse(source)
	if err != nil || srcURL.Scheme != "" || srcURL.Host != "" || strings.HasPrefix(source, "//") || !IsSupportedRasterPath(srcURL.Path) {
		return "", false
	}

	srcPath := path.Clean("/" + srcURL.Path)
	diskPath := filepath.Join(sourceDir, filepath.FromSlash(strings.TrimPrefix(srcPath, "/")))

	file, err := os.Open(diskPath)
	if err != nil {
		return "", false
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return "", false
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	dimensionsToGenerate := FindFinalDimensions(width, height)

	srcsetParts := []string{}
	for _, size := range dimensionsToGenerate {
		ext := path.Ext(srcPath)
		baseFileName := srcPath[:len(srcPath)-len(ext)]
		generatedURL := &url.URL{
			Path:     baseFileName + "." + strconv.Itoa(size.W) + ".webp",
			RawQuery: srcURL.RawQuery,
			Fragment: srcURL.Fragment,
		}
		locationToUse := generatedURL.String()
		srcsetParts = append(srcsetParts, locationToUse+" "+strconv.Itoa(size.W)+"w")
	}

	return strings.Join(srcsetParts, ", "), len(srcsetParts) > 0
}
