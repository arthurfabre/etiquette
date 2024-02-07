package etiquette

import (
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	"go.afab.re/etiquette/monochrome"
)

type Bounds struct {
	// Dx is the exact width the image must be, in pixels.
	Dx int
	// Dy is the minimum height of the image, in pixels.
	MinDy int
}

type TextOpts struct {
	DPI  int
	Font *opentype.Font
}

// Text renders text as an image suitable for printing.
func Text(b Bounds, text string, opts TextOpts) (*monochrome.Image, error) {
	// We're going to rotate the label to print it landscape, it's height needs to match
	// the width of the printer.
	height := px(b.Dx)

	face, err := face(height, opts)
	if err != nil {
		return nil, err
	}

	dst := image.NewGray(bounds(height, face, text))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	d := font.Drawer{
		Dst:  dst,
		Src:  image.Black,
		Face: face,
	}
	d.DrawString(text)

	return Image(b, landscape(dst))
}

// Rotate image 90° clockwise.
func landscape(img *image.Gray) *image.Gray {
	dst := image.NewGray(image.Rect(
		img.Bounds().Min.Y, img.Bounds().Min.X,
		img.Bounds().Max.Y, img.Bounds().Max.X,
	))

	// Use bounds of each img independently in case Min was not (0, 0).
	for x := 0; x < dst.Bounds().Dx(); x++ {
		for y := 0; y < dst.Bounds().Dy(); y++ {
			dst.Set(
				dst.Bounds().Min.X+x,
				dst.Bounds().Min.Y+y,
				img.At(
					img.Bounds().Max.X-1-y,
					img.Bounds().Min.Y+x,
				),
			)
		}
	}

	return dst
}

type px int

// Find the biggest font for a given height.
func face(height px, opts TextOpts) (font.Face, error) {
	var best font.Face

	for i := float64(1); ; i++ {
		face, err := opentype.NewFace(opts.Font, &opentype.FaceOptions{
			Size:    i,
			DPI:     float64(opts.DPI),
			Hinting: font.HintingFull,
		})
		if err != nil {
			return nil, err
		}

		if face.Metrics().Height.Ceil() > int(height) {
			return best, nil
		}

		best = face
	}
}

func bounds(height px, face font.Face, text string) image.Rectangle {
	m := face.Metrics()

	margin := 7

	// Margin to center the font vertically.
	// (not the specific text - otherwise different labels will end up aligned differently).
	yMin := -m.Ascent.Ceil()
	yMax := m.Descent.Ceil()

	yMargin := int(height) - (yMax - yMin)

	// If the margin isn't a multiple of two, (arbitrarily) give the extra space to yMax.
	yMax += (yMargin + 1) / 2
	yMin -= yMargin / 2

	// Combine font based vertical bounds, and text based horizontal bounds.
	tBounds, _ := font.BoundString(face, text)
	return image.Rect(
		tBounds.Min.X.Floor()-margin, yMin,
		tBounds.Max.X.Ceil()+margin, yMax,
	)
}
