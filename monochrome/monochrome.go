package monochrome

import (
	"image"
	"image/color"
)

func Model() color.Palette {
	return color.Palette{color.White, color.Black}
}

// Image is an image with black data on a white background.
type Image struct {
	// We could represent it much more efficiently as we only need a single bit per pixel,
	// but this is easier for now.
	p *image.Paletted
}

// Make sure we implement PalettedImage - some encoders like png,
// handle PalettedImages with two colors and encode them as 1 bit images.
var _ image.PalettedImage = &Image{}

func New(r image.Rectangle) *Image {
	return &Image{
		p: image.NewPaletted(r, Model()),
	}
}

func (m *Image) ColorModel() color.Model {
	return m.p.ColorModel()
}

func (m *Image) Bounds() image.Rectangle {
	return m.p.Bounds()
}

func (m *Image) At(x, y int) color.Color {
	return m.p.At(x, y)
}

func (m *Image) ColorIndexAt(x, y int) uint8 {
	return m.p.ColorIndexAt(x, y)
}

func (m *Image) BlackAt(x, y int) bool {
	return m.p.ColorIndexAt(x, y) == 1
}

func (m *Image) SetBlack(x, y int, isBlack bool) {
	if isBlack {
		m.p.SetColorIndex(x, y, 1)
	} else {
		m.p.SetColorIndex(x, y, 0)
	}
}
