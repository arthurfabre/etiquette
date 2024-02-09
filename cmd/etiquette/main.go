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

	"golang.org/x/exp/maps"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"go.afab.re/etiquette"
	"go.afab.re/etiquette/monochrome"
	_ "go.afab.re/etiquette/pt700"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `%s [options] /dev/usb/lpN

Print each line from stdin as a text label on a Brother PT-700 printer connected as /dev/usb/lpN.

`, os.Args[0])
		flag.PrintDefaults()
	}

	var (
		media   = flag.Bool("media", false, "Show media loaded in printer only, don't print.")
		img     = flag.Bool("img", false, "Print an image (PNG/GIF/JPEG) from stdin instead of text.")
		preview = flag.String("preview", "", "Preview the print as a PNG image written to filename, don't print.")
		printer = flag.String("printer", "", "Path to the printer of the form /dev/usb/lpN.")
		list    = flag.Bool("list", false, "List connected supported printers only, don't print.")
	)
	flag.Parse()

	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(-1)
	}

	if err := print(os.Stdin, flags{
		media:   *media,
		img:     *img,
		preview: *preview,
		printer: *printer,
		list:    *list,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(-1)
	}
}

type flags struct {
	media   bool
	img     bool
	preview string
	printer string
	list    bool
}

func print(labels io.Reader, flags flags) error {
	var printer etiquette.Printer
	switch {
	case flags.printer == "", flags.list:
		printers, err := etiquette.Printers()
		if err != nil {
			return err
		}

		if flags.list {
			fmt.Println(maps.Keys(printers))
			return nil
		}

		if len(printers) == 0 {
			return fmt.Errorf("no supported printers found")
		}

		if len(printers) != 1 {
			// TODO - close all printers?
			return fmt.Errorf("more than one printer found, select one with -printer: %v", maps.Keys(printers))
		}
		for _, p := range printers {
			printer = p
		}
	default:
		var err error
		printer, err = etiquette.OpenPrinter(flags.printer)
		if err != nil {
			return err
		}
	}
	defer printer.Close()

	info, err := printer.Info()
	if err != nil {
		return err
	}
	if flags.media {
		fmt.Println(info.MediaName)
		return nil
	}

	var imgs []*monochrome.Image
	if flags.img {
		imgs, err = img(info, labels)
	} else {
		imgs, err = text(info, labels)
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

func img(info etiquette.PrinterInfo, labels io.Reader) ([]*monochrome.Image, error) {
	img, _, err := image.Decode(labels)
	if err != nil {
		return nil, err
	}

	mono, err := etiquette.Image(info.Bounds, img)
	if err != nil {
		return nil, err
	}

	return []*monochrome.Image{mono}, nil
}

func text(info etiquette.PrinterInfo, labels io.Reader) ([]*monochrome.Image, error) {
	ft, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}

	var imgs []*monochrome.Image

	scanner := bufio.NewScanner(labels)
	for scanner.Scan() {
		img, err := etiquette.Text(info.Bounds, scanner.Text(), etiquette.TextOpts{
			Font: ft,
			DPI:  info.DPI,
		})
		if err != nil {
			return nil, err
		}

		imgs = append(imgs, img)
	}

	return imgs, nil
}

func printPrinters(w io.Writer, printers map[string]etiquette.Printer) error {

	return nil
}
