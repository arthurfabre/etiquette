package etiquette

import (
	"fmt"
	"image"
	"image/draw"

	"go.afab.re/etiquette/monochrome"
)

// Image converts an image to one suitable for printing:
// - Monochrome.
// - Padded out to bounds.
func Image(b Bounds, img image.Image) (*monochrome.Image, error) {
	return pad(b, monochrome.From(img))
}

func pad(b Bounds, src *monochrome.Image) (*monochrome.Image, error) {
	if src.Bounds().Dx() > b.Dx {
		return nil, fmt.Errorf("expected up to %dpx wide image but got %dx", b.Dx, src.Bounds().Dx())
	}
	xPadding := b.Dx - src.Bounds().Dx()

	yPadding := b.MinDy - src.Bounds().Dy()
	if src.Bounds().Dy() >= b.MinDy {
		yPadding = 0
	}

	dst := monochrome.New(image.Rectangle{
		// If padding isn't a multiple of two, give it to the top left.
		src.Bounds().Min.Sub(image.Pt((xPadding+1)/2, (yPadding+1)/2)),
		src.Bounds().Max.Add(image.Pt(xPadding/2, yPadding/2)),
	})
	draw.Draw(dst, src.Bounds(), src, src.Bounds().Min, draw.Src)

	return dst, nil
}
