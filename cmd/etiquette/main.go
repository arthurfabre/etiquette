package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"go.afab.re/etiquette"
	"go.afab.re/etiquette/monochrome"
	"go.afab.re/etiquette/pt700"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `%s [options] /dev/usb/lpN

Print each line from stdin as a text label on a Brother PT-700 printer connected as /dev/usb/lpN.

`, os.Args[0])
		flag.PrintDefaults()
	}

	var (
		status  = flag.Bool("status", false, "Show printer status only, don't print anything.")
		img     = flag.Bool("img", false, "Print an image (PNG/GIF/JPEG) from stdin instead of text.")
		preview = flag.String("preview", "", "Preview the print as a PNG image written to filename.")
	)
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(-1)
	}

	if err := print(flag.Arg(0), os.Stdin, flags{
		status:  *status,
		img:     *img,
		preview: *preview,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(-1)
	}
}

type flags struct {
	status  bool
	img     bool
	preview string
}

func print(printerPath string, labels io.Reader, flags flags) error {
	printer, err := pt700.Open(printerPath)
	if err != nil {
		return err
	}
	defer printer.Close()

	status, err := printer.Status()
	if err != nil {
		return err
	}
	if flags.status {
		fmt.Printf("%+v\n", status)
		return nil
	}

	if err := status.Err(); err != nil {
		return err
	}

	dx, err := status.MediaWidth.Dx()
	if err != nil {
		return err
	}
	bounds := etiquette.Bounds{
		Dx:    dx,
		MinDy: status.MediaWidth.MinDy(),
	}

	var imgs []*monochrome.Image
	if flags.img {
		imgs, err = img(bounds, labels)
	} else {
		imgs, err = text(bounds, status.MediaWidth.DPI(), labels)
	}
	if err != nil {
		return err
	}

	if flags.preview != "" {
		if len(imgs) != 1 {
			return fmt.Errorf("preview only supported with a single label")
		}

		preview, err := os.Create(flags.preview)
		if err != nil {
			return err
		}

		return png.Encode(preview, imgs[0])
	}

	return printer.Print(imgs...)
}

func img(b etiquette.Bounds, labels io.Reader) ([]*monochrome.Image, error) {
	img, _, err := image.Decode(labels)
	if err != nil {
		return nil, err
	}

	mono, err := etiquette.Image(b, img)
	if err != nil {
		return nil, err
	}

	return []*monochrome.Image{mono}, nil
}

func text(b etiquette.Bounds, dpi int, labels io.Reader) ([]*monochrome.Image, error) {
	ft, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}

	var imgs []*monochrome.Image

	scanner := bufio.NewScanner(labels)
	for scanner.Scan() {
		img, err := etiquette.Text(b, scanner.Text(), etiquette.TextOpts{
			Font: ft,
			DPI:  dpi,
		})
		if err != nil {
			return nil, err
		}

		imgs = append(imgs, img)
	}

	return imgs, nil
}
