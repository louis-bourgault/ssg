package image

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
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

func FindFinalDimensions(ow int, oh int) []types.ImageSize { //ow, oh mean original width and height. This function returns a list of the dimensions that this will be transformed into.
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

func GenerateImages(originalPath string, finalPath string) {
	file, err := os.Open(originalPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	fmt.Println("About to decode: ", originalPath)

	img, _, err := image.Decode(file)
	if err != nil {
		panic(err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	dimensionsToGenerate := FindFinalDimensions(width, height)
	for dimension, _ := range dimensionsToGenerate {
		size := dimensionsToGenerate[dimension]
		resized := resize.Resize(uint(size.W), uint(size.H), img, resize.Lanczos3)
		ext := filepath.Ext(finalPath)
		baseFileName := finalPath[:len(finalPath)-len(ext)]
		locationToSave := baseFileName + "." + strconv.Itoa(size.W) + ".webp"
		outputFile, err := os.Create(locationToSave)
		if err != nil {
			panic(err)
		}
		defer outputFile.Close()
		options, _ := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 75) //75 is the quality, we'll see how this goes
		err = webp.Encode(outputFile, resized, options)
		if err != nil {
			panic(err)
		}
	}

}

func AdaptImgTag(originalElement string, isFirst bool) string {
	//TODO: This function is really slow for what it does, which is a risk for things like the Cloudflare 20 minute site build. A single run for a large image often takes multiple seconds.
	// It may be better to go through the directory and pattern match the name (since they're already generated) instead of deterministically figuring out what resolutions it would be from scratch

	fmt.Println("we got sent", originalElement)
	//in another file, we'll have a regex that finds all image tags and adapts them to use srcset
	// we want to take the original tag, which includes a reference to the original src, locate the file and figure out its dimensions (we are guaranteed that the images are already generated) and splice in the srcset.
	//<img src="path/to/image.jpg" alt="description"> might be our input here
	srcRegex := regexp.MustCompile(`src="([^"]+)"`)
	matches := srcRegex.FindStringSubmatch(originalElement)
	if len(matches) < 2 {
		panic("Could not find src attribute in img tag")
	}
	if strings.HasPrefix(matches[1], "https://") || strings.HasPrefix(matches[1], "http://") {
		//we don't handle external images for now
		return originalElement
	}
	srcPath := matches[1]

	file, err := os.Open("routes" + srcPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		panic(err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	dimensionsToGenerate := FindFinalDimensions(width, height)

	srcsetParts := []string{}
	for dimension := range dimensionsToGenerate {
		size := dimensionsToGenerate[dimension]
		ext := filepath.Ext(srcPath)
		baseFileName := srcPath[:len(srcPath)-len(ext)]
		locationToUse := baseFileName + "." + strconv.Itoa(size.W) + ".webp"
		srcsetParts = append(srcsetParts, locationToUse+" "+strconv.Itoa(size.W)+"w")
	}

	srcset := ""
	for i, part := range srcsetParts {
		srcset += part
		if i < len(srcsetParts)-1 {
			srcset += ", "
		}
	}

	trimmed := strings.TrimRight(originalElement, "/>")
	trimmed = strings.TrimSpace(trimmed)
	var additionalTags string
	if !isFirst {
		additionalTags = " loading=\"lazy\" decoding=\"async\""
	}
	trimmed += additionalTags
	modifiedElement := trimmed + " srcset=\"" + srcset + "\" />"
	return modifiedElement
}
