package monochrome

import (
	"image"
	"image/color"
	"image/draw"
)

// From converts an image to monochrome with Otsu thresholding.
// https://en.wikipedia.org/wiki/Otsu%27s_method
func From(img image.Image) *Image {
	// First get a grayscale image.
	var gray *image.Gray
	switch i := img.(type) {
	case *Image:
		return i
	case *image.Gray:
		gray = i
	default:
		gray = image.NewGray(img.Bounds())
		draw.Draw(gray, gray.Bounds(), img, img.Bounds().Min, draw.Src)
	}

	// Then figure out the threshold and turn it into monochrome
	threshold := otsuThreshold(gray)

	mono := New(gray.Bounds())

	for x := gray.Bounds().Min.X; x < gray.Bounds().Max.X; x++ {
		for y := gray.Bounds().Min.Y; y < gray.Bounds().Max.Y; y++ {
			mono.SetBlack(x, y, gray.GrayAt(x, y).Y <= threshold)
		}
	}

	return mono
}

func otsuThreshold(i *image.Gray) uint8 {
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
