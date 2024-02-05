package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	"io"
	"os"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"go.afab.re/etiquette"
	"go.afab.re/etiquette/otsu"
	"go.afab.re/etiquette/pt700"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `%s [options] /dev/usb/lpN

Print each line from stdin as a text label on a Brother PT-700 printer connected as /dev/usb/lpN.

`, os.Args[0])
		flag.PrintDefaults()
	}

	var status = flag.Bool("status", false, "Show printer status only, don't print anything.")
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(-1)
	}

	if err := print(flag.Arg(0), *status, os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(-1)
	}
}

func print(printerPath string, printStatus bool, labels io.Reader) error {
	printer, err := pt700.Open(printerPath)
	if err != nil {
		return err
	}
	defer printer.Close()

	status, err := printer.Status()
	if err != nil {
		return err
	}
	if printStatus {
		fmt.Printf("%+v\n", status)
		return nil
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
