package main

import (
	"fmt"
	"os"

	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"

	"afab.re/etiquette"
	"afab.re/etiquette/otsu"
	"afab.re/etiquette/pt700"
)

func main() {
	if err := print(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(-1)
	}
}

func print(printerPath string, text string) error {
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

	ft, err := opentype.Parse(gomono.TTF)
	if err != nil {
		return err
	}

	img, err := etiquette.Render(text, etiquette.Opts{
		Font:   ft,
		Height: etiquette.Px(heightPx),
		DPI:    status.MediaWidth.DPI(),
	})
	if err != nil {
		return err
	}

	monochrome := otsu.Otsu(img)

	return printer.Print(monochrome)
}
