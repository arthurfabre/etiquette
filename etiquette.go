package etiquette

import (
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

type Px int

type Opts struct {
	Height Px
	DPI    int
	Font   *opentype.Font
}

func Render(text string, opts Opts) (*image.Gray, error) {
	face, err := face(opts)
	if err != nil {
		return nil, err
	}

	dst := image.NewGray(bounds(opts.Height, face, text))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	d := font.Drawer{
		Dst:  dst,
		Src:  image.Black,
		Face: face,
	}
	d.DrawString(text)

	return dst, nil
}

// Find the biggest font for a given height.
func face(opts Opts) (font.Face, error) {
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

		if face.Metrics().Height.Ceil() > int(opts.Height) {
			return best, nil
		}

		best = face
	}
}

func bounds(height Px, face font.Face, text string) image.Rectangle {
	m := face.Metrics()

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
		tBounds.Min.X.Floor(), yMin,
		tBounds.Max.X.Ceil(), yMax,
	)
}
