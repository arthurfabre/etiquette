package pt700

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"golang.org/x/sys/unix"

	"go.afab.re/etiquette"
	"go.afab.re/etiquette/monochrome"
)

const (
	vendorID  = 0x04F9
	productID = 0x2061
)

func init() {
	etiquette.RegisterPrinter(vendorID, productID, func(path string) (etiquette.Printer, error) {
		var (
			p   etiquette.Printer
			err error
		)
		p, err = Open(path)
		return p, err
	})
}

// PT700 controls a Brother PT-700 label printer on Linux through the usblp driver.
type PT700 int // We have to poll() to read responses, it's easier to use a raw FD.

// Open opens a PT700 printer. Path should be of the form /dev/usb/lpN.
func Open(path string) (PT700, error) {
	fd, err := unix.Open(path, unix.O_RDWR, 0)
	return PT700(fd), err
}

// Print the images as pages of one job so the ~24.5mm of blank start tape is only needed once.
// The images will be individually cut.
func (p PT700) Print(imgs ...*monochrome.Image) error {
	if err := p.reset(); err != nil {
		return err
	}

	// Initialize. This seems to start the print job.
	if err := p.write([]byte{0x1B, 0x40}); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	// Docs says we need to Status() at least once, do it just in case.
	// We can also check the tape size in case it has changed.
	status, err := p.status()
	if err != nil {
		return err
	}
	if err := status.Err(); err != nil {
		return err
	}

	if err := checkImgs(status.MediaWidth, imgs...); err != nil {
		return err
	}

	// Actually print.
	for i, img := range imgs {
		var pos pagePos
		if i == 0 {
			pos = pos | first
		}
		if i == len(imgs)-1 {
			pos = pos | last
		}

		if err := p.printPage(status.MediaWidth, pos, img); err != nil {
			return fmt.Errorf("printing page %d: %w", i, err)
		}
	}

	return nil
}

func (p PT700) reset() error {
	// Invalidate. Brother docs 2.1.1 sends 100 bytes, so we do too.
	if err := p.write(make([]byte, 100)); err != nil {
		return fmt.Errorf("invalidate: %w", err)
	}

	// Discard any leftover junk we or other programs didn't read.
	return readUntilEOF(int(p))
}

func checkImgs(width MediaWidth, imgs ...*monochrome.Image) error {
	dx, err := width.Dx()
	if err != nil {
		return err
	}
	minDy := width.MinDy()

	for _, img := range imgs {
		if img.Bounds().Dx() != dx {
			return fmt.Errorf("printer has %v tape, expected %dpx wide image but got %dx", width, dx, img.Bounds().Dx())
		}
		if img.Bounds().Dy() < minDy {
			return fmt.Errorf("printer can't print images shorter than %dpx, got %dpx", minDy, img.Bounds().Dy())
		}
	}

	return nil
}

// Position of page in job.
// Can be both first and last if it's the only page.
type pagePos int

const (
	middle pagePos = 0
	first  pagePos = 1 << iota
	last
)

func (p PT700) printPage(width MediaWidth, pos pagePos, img *monochrome.Image) error {
	// Not the first page? Wait for "Waiting to receive"
	if pos&first == 0 {
		if _, err := p.readStatus(StatusPhaseChange); err != nil {
			return err
		}
	}

	// Control codes (Brother PDF 2.1.2).
	// Raster mode.
	if err := p.write([]byte{0x1B, 0x69, 0x61, 0x01}); err != nil {
		return fmt.Errorf("enabling raster mode: %w", err)
	}

	// Print information
	info := []byte{
		0x1B, 0x69, 0x7A,
		// Only validate the media width in case the tape has changed,
		// the type and length don't really matter,
		// PrinterRecovery should always be on according to the manual.
		0x84,
		0x00,        // Type, we don't request validation.
		byte(width), // Media width mm.
		0x00,        // Media length mm, we don't request validation.
	}
	// Number of raster lines (height of image).
	info = binary.LittleEndian.AppendUint32(info, uint32(img.Bounds().Dy()))
	// Starting page or not.
	if pos&first != 0 {
		info = append(info, 0x00)
	} else {
		info = append(info, 0x01)
	}
	// n10: always 0.
	info = append(info, 0x00)
	if err := p.write(info); err != nil {
		return fmt.Errorf("print information: %w", err)
	}

	// Mode settings.
	if err := p.write([]byte{
		0x1B, 0x69, 0x4D,
		// Enable auto cut.
		0x40,
	}); err != nil {
		return fmt.Errorf("mode settings: %w", err)
	}

	// Advanced mode settings.
	if err := p.write([]byte{
		0x1B, 0x69, 0x4B,
		// "Chain-printing" lets the printer print several jobs in a row,
		// by not feeding out the label and cutting it for the last page.
		// We print all the labels as pages of one job, so we actually don't want chain-printing.
		0x08,
	}); err != nil {
		return fmt.Errorf("advanced mode settings: %w", err)
	}

	// Margin.
	if err := p.write([]byte{
		// Manual says min 2mm margins (14 dots) in 2.3.3. But 0 seems to work fine.
		// Lets the caller deal with margins themselves.
		0x1B, 0x69, 0x64, 0x00, 0x00,
	}); err != nil {
		return fmt.Errorf("margins: %w", err)
	}

	// Compression.
	if err := p.write([]byte{
		0x4D,
		// No compression.
		0x00,
	}); err != nil {
		return fmt.Errorf("compression: %w", err)
	}

	// Raster data.
	if err := p.printRaster(width, img); err != nil {
		return fmt.Errorf("raster: %w", err)
	}

	// Print.
	printCmd := byte(0x0C)
	if pos&last != 0 {
		// Print and feed.
		printCmd = 0x1A
	}
	if err := p.write([]byte{printCmd}); err != nil {
		return fmt.Errorf("print: %w", err)
	}

	// First "Printing".
	if _, err := p.readStatus(StatusPhaseChange); err != nil {
		return err
	}

	if pos&last != 0 {
		// "Feeding".
		if _, err := p.readStatus(StatusPhaseChange); err != nil {
			return err
		}
	}

	// Finally "Printing completed".
	_, err := p.readStatus(StatusPrintingCompleted)
	return err
}

func (p PT700) printRaster(width MediaWidth, img *monochrome.Image) error {
	// Print bottom line first.
	for y := img.Bounds().Max.Y; y > img.Bounds().Min.Y; y-- {
		if err := p.rasterLine(width, img, y); err != nil {
			return err
		}
	}

	return nil
}

func (p PT700) rasterLine(width MediaWidth, img *monochrome.Image, y int) error {
	const totalPins = 128

	line := make([]byte, totalPins/8)

	// Only the middle pins are used for printing, offset everything.
	pin, err := width.unusedPins(totalPins)
	if err != nil {
		return err
	}

	for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
		byt := pin / 8
		bit := pin % 8

		if img.BlackAt(x, y) {
			line[byt] = line[byt] | (1<<7)>>bit
		}

		pin++
	}

	// Manual says 0x67! But that doesn't work, and the example
	// in 2.2.3 uses 0x47.
	return p.write(append([]byte{0x47, 16, 0}, line...))
}

func (p PT700) Info() (etiquette.PrinterInfo, error) {
	status, err := p.Status()
	if err != nil {
		return etiquette.PrinterInfo{}, err
	}

	dx, err := status.MediaWidth.Dx()
	if err != nil {
		return etiquette.PrinterInfo{}, err
	}

	return etiquette.PrinterInfo{
		MediaName: status.MediaWidth.String(),
		Bounds: etiquette.Bounds{
			Dx:    dx,
			MinDy: status.MediaWidth.MinDy(),
		},
		// Brother PDF 2.3.4
		DPI: 180,
	}, nil
}

func (p *PT700) Status() (Status, error) {
	if err := p.reset(); err != nil {
		return Status{}, err
	}

	return p.status()
}

// Status() but without reset().
func (p PT700) status() (Status, error) {
	if err := p.write([]byte{0x1B, 0x69, 0x53}); err != nil {
		return Status{}, fmt.Errorf("status write: %w", err)
	}

	return p.readStatus(StatusReplyToRequest)
}

func (p PT700) readStatus(expectedType StatusType) (Status, error) {
	resp := make([]byte, 32)
	if err := p.read(resp, time.Second*10); err != nil {
		return Status{}, fmt.Errorf("status read: %w", err)
	}

	s := Status{
		Err1:       Error1(resp[8]),
		Err2:       Error2(resp[9]),
		MediaWidth: MediaWidth(resp[10]),
		MediaType:  MediaType(resp[11]),
		Type:       StatusType(resp[18]),
		Phase:      PhaseType(resp[19]),
	}

	if s.Type != expectedType {
		return Status{}, fmt.Errorf("expected status type %v got %+v", expectedType, s)
	}
	return s, nil
}

func (p PT700) write(b []byte) error {
	for wrote := 0; wrote != len(b); {
		n, err := unix.Write(int(p), b[wrote:])
		switch {
		case errors.Is(unix.EINTR, err):
			continue
		case err != nil:
			return fmt.Errorf("write: %w", err)
		}

		wrote += n
	}

	return nil
}

// io.ReadFull() but will poll().
func (p PT700) read(buf []byte, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	pollFds := []unix.PollFd{
		{Fd: int32(p), Events: unix.POLLIN},
	}

	for read := 0; read < len(buf); {
		// Negative timeout is an infinite timeout for poll().
		remaining := time.Until(deadline).Milliseconds()
		switch {
		case remaining < 0:
			return os.ErrDeadlineExceeded
		case remaining > math.MaxInt:
			return fmt.Errorf("timeout too big")
		}

		n, err := unix.Poll(pollFds, int(remaining))
		switch {
		case errors.Is(err, unix.EINTR):
			continue
		case err != nil:
			return err
		case n == 0:
			return os.ErrDeadlineExceeded
		case (pollFds[0].Revents & unix.POLLNVAL) != 0:
			return fmt.Errorf("POLLNVAL")
		case (pollFds[0].Revents & unix.POLLERR) != 0,
			(pollFds[0].Revents & unix.POLLHUP) != 0:
			return fmt.Errorf("printer disconnected")
		case (pollFds[0].Revents & unix.POLLIN) == 0:
			return fmt.Errorf("poll() returned but no data, n %d, revents: %x", n, pollFds[0].Revents)
		}

		n, err = readFull(int(p), buf[read:])
		if err != nil {
			return err
		}
		read += n
	}

	return nil
}

// readFull reads up to len(b) bytes, or EOF from fd.
// On EOF no error is returned.
func readFull(fd int, b []byte) (int, error) {
	read := 0

	for read != len(b) {
		n, err := unix.Read(fd, b[read:])
		switch {
		case errors.Is(unix.EINTR, err):
			continue
		case err != nil:
			return 0, err
		// EOF.
		case n == 0:
			// Sometimes we seem to get spurious poll() events when there's nothing
			// actually available to read.
			// There are also no guarantees the full len(b) are immediately available to read.
			return read, nil
		}

		read += n
	}

	return read, nil
}

func readUntilEOF(fd int) error {
	b := make([]byte, 128)

	for {
		n, err := unix.Read(fd, b)
		switch {
		case errors.Is(unix.EINTR, err):
			continue
		case err != nil:
			return err
		// EOF.
		case n == 0:
			return nil
		}
	}
}

func (p PT700) Close() error {
	return unix.Close(int(p))
}
