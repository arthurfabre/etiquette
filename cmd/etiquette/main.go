package main

import (
	"bufio"
	"fmt"
	"image"
	"io"
	"os"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"afab.re/etiquette"
	"afab.re/etiquette/otsu"
	"afab.re/etiquette/pt700"
)

func main() {
	if err := print(os.Args[1], os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(-1)
	}
}

func print(printerPath string, labels io.Reader) error {
	printer, err := pt700.Open(printerPath)
	if err != nil {
		return err
	}
	defer printer.Close()

	status, err := printer.Status()
	if err != nil {
		return err
	}
	if err := status.Err(); err != nil {
		return err
	}

	heightPx, err := status.MediaWidth.Px()
	if err != nil {
		return err
	}

	ft, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return err
	}

	var imgs []image.PalettedImage
	scanner := bufio.NewScanner(labels)
	for scanner.Scan() {
		img, err := etiquette.Render(scanner.Text(), etiquette.Opts{
			Font:   ft,
			Height: etiquette.Px(heightPx),
			DPI:    status.MediaWidth.DPI(),
		})
		if err != nil {
			return err
		}

		imgs = append(imgs, otsu.Otsu(img))
	}

	return printer.Print(imgs...)
}
