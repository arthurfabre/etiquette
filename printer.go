package etiquette

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"unsafe"

	"golang.org/x/sys/unix"

	"go.afab.re/etiquette/monochrome"
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

// Printers lists the available printers.
func Printers() (map[string]Printer, error) {
	devs, err := fs.Glob(os.DirFS("/dev/usb"), "lp[0-9]*")
	if err != nil {
		return nil, err
	}

	printers := make(map[string]Printer)
	for _, dev := range devs {
		printer, err := OpenPrinter(path.Join("/dev/usb", dev))
		switch {
		case errors.Is(err, errors.ErrUnsupported):
			// dsf
		case err != nil:
			return nil, fmt.Errorf("%s: %w", dev, err)
		default:
			printers[dev] = printer
		}
	}

	return printers, nil
}

type usbID struct {
	vendorID  uint16
	productID uint16
}

var constructors = map[usbID]func(path string) (Printer, error){}

func RegisterPrinter(vendorID uint16, productID uint16, constructor func(path string) (Printer, error)) {
	constructors[usbID{vendorID, productID}] = constructor
}

func OpenPrinter(path string) (Printer, error) {
	const (
		IOC_NRBITS   = 8
		IOC_TYPEBITS = 8
		// TODO - this is not portable across architectures!
		// Some have different SIZEBITS and DIRBITS
		IOC_SIZEBITS = 13
		IOC_DIRBITS  = 3

		IOC_NRSHIFT   = 0
		IOC_TYPESHIFT = (IOC_NRSHIFT + IOC_NRBITS)
		IOC_SIZESHIFT = (IOC_TYPESHIFT + IOC_TYPEBITS)
		IOC_DIRSHIFT  = (IOC_SIZESHIFT + IOC_SIZEBITS)

		IOC_WRITE = 1
		IOC_READ  = 2

		IOCNR_GET_VID_PID = 6
	)

	fd, err := unix.Open(path, unix.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	// This IOCTL is an platform dependent mess. It expects [2]int, but assumes int == uint16.
	//
	var id struct {
		vendorID  uint16
		_         uint16
		productID uint16
		_         uint16
	}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(
		IOCNR_GET_VID_PID<<IOC_NRSHIFT|
			'P'<<IOC_TYPESHIFT|
			unsafe.Sizeof(id)<<IOC_SIZESHIFT|
			IOC_READ<<IOC_DIRSHIFT,
	), uintptr(unsafe.Pointer(&id)))
	if errno != 0 {
		return nil, fmt.Errorf("GET_VID_PID: %w", err)
	}
	fmt.Printf("ID for %s is %04x:%04x\n", path, id.vendorID, id.productID)

	constructor, ok := constructors[usbID{
		vendorID:  id.vendorID,
		productID: id.productID,
	}]
	if !ok {
		fmt.Println("not found...")
		return nil, errors.ErrUnsupported
	}

	// TODO - otherwise we get EBUSY
	unix.Close(fd)

	// TODO - should constructors take a raw FD?
	return constructor(path)
}
