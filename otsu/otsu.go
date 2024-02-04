package otsu

import (
	"image"
	"image/color"
)

// Otsu converts a grayscale image to monochrome with Otsu thresholding.
// https://en.wikipedia.org/wiki/Otsu%27s_method
// Returns a PalettedImage with Black and White only, which some encoders (like
// png) treat specially and encode as a 1bit image.
func Otsu(i *image.Gray) image.PalettedImage {
	threshold := threshold(i)

	dst := image.NewPaletted(i.Bounds(), color.Palette{color.Black, color.White})

	for x := i.Bounds().Min.X; x < i.Bounds().Max.X; x++ {
		for y := i.Bounds().Min.Y; y < i.Bounds().Max.Y; y++ {
			if i.At(x, y).(color.Gray).Y > threshold {
				dst.SetColorIndex(x, y, 1)
			} else {
				dst.SetColorIndex(x, y, 0)
			}
		}
	}

	return dst
}

func threshold(i *image.Gray) uint8 {
	histo := intensityHistogram(i)

	totalPixels := i.Bounds().Dx() * i.Bounds().Dy()

	var totalWeightedSum int
	for threshold, pixels := range histo {
		totalWeightedSum += threshold * pixels
	}

	var (
		// Best threshold and inter-class variance so far.
		bestThreshold uint8
		bestVariance  int

		// How many black pixels are <= threshold.
		blkPixels int
		// Mean intensity of black pixels.
		blkWeightedSum int
	)
	for threshold, pixels := range histo {
		blkPixels += pixels
		blkWeightedSum += threshold * pixels

		wtePixels := totalPixels - blkPixels
		wteWeightedSum := totalWeightedSum - blkWeightedSum

		// Avoid division by 0. All the pixels are the same color so far,
		// so this threshold won't be any better anyways.
		if blkPixels == 0 || wtePixels == 0 {
			continue
		}

		blkMean := blkWeightedSum / blkPixels
		wteMean := wteWeightedSum / wtePixels

		variance := blkPixels * wtePixels * square(blkMean-wteMean)
		if variance > bestVariance {
			bestVariance = variance
			bestThreshold = uint8(threshold)
		}
	}

	return bestThreshold
}

func intensityHistogram(i *image.Gray) [256]int {
	var histo [256]int

	for x := i.Bounds().Min.X; x < i.Bounds().Max.X; x++ {
		for y := i.Bounds().Min.Y; y < i.Bounds().Max.Y; y++ {
			histo[i.At(x, y).(color.Gray).Y]++
		}
	}

	return histo
}

func square(a int) int {
	return a * a
}
