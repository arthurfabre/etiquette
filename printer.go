package etiquette

import (
	"fmt"
	"io"

	"go.afab.re/etiquette/monochrome"
	"go.afab.re/etiquette/usblp"
)

type Printer interface {
	io.Closer

	Info() (PrinterInfo, error)
	Print(imgs ...*monochrome.Image) error
}

type PrinterInfo struct {
	// Human readable name of the loaded media.
	MediaName string

	Bounds Bounds

	DPI int
}

var registered = map[usblp.ID]func(*usblp.Device) (Printer, error){}

func RegisterPrinter(id usblp.ID, new func(*usblp.Device) (Printer, error)) {
	registered[id] = new
}

// Printers lists the available printers.
func Printers() (map[string]Printer, error) {
	connected, err := usblp.Connected()
	if err != nil {
		return nil, err
	}

	printers := make(map[string]Printer)
	for _, dev := range connected {
		id, err := dev.ID()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dev.Path(), err)
		}

		constructor, ok := registered[id]
		if !ok {
			continue
		}

		printer, err := constructor(dev)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dev.Path(), err)
		}

		printers[dev.Path()] = printer
	}

	return printers, nil
}
