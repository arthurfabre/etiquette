package pt700

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// PT700 controls a Brother PT-700 label printer on Linux through the usblp driver.
type PT700 int // We have to poll() to read responses, it's easier to use a raw FD.

// Open opens a PT700 printer. Path should be of the form /dev/usb/lpN.
func Open(path string) (PT700, error) {
	fd, err := unix.Open(path, unix.O_RDWR, 0)
	return PT700(fd), err
}

// Print "chain-prints" the images as one job so the ~24.5mm of blank start tape is only needed once.
// The images will be individually cut.
func (p PT700) Print(imgs ...image.PalettedImage) error {
	if err := p.invalidate(); err != nil {
		return err
	}

	// Initialize.
	if err := p.write([]byte{0x1B, 0x40}); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	// Docs says we need to Status() at least once, do it just in case.
	// We can also check the tape size in case it has changed.
	// TODO - do we get an error if the tape door is openned after this point?
	status, err := p.Status()
	if err != nil {
		return err
	}
	if err := status.Err(); err != nil {
		return err
	}

	px, err := status.Width.Px()
	if err != nil {
		return err
	}

	for _, img := range imgs {
		// TODO - check color model too! Need to iterate over it, and check it's only White or Black.

		if gotPx := Px(img.Bounds().Dy()); gotPx != px {
			return fmt.Errorf("printer has %v tape, expected %dpx img but got %dpx", status.Width, px, gotPx)
		}
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

		if err := p.printPage(status.Width, pos, img); err != nil {
			return fmt.Errorf("printing page %d: %w", i, err)
		}
	}

	return nil
}

func (p PT700) invalidate() error {
	// Invalidate. Brother docs 2.1.1 sends 100 bytes, so we do too.
	if err := p.write(make([]byte, 100)); err != nil {
		return fmt.Errorf("invalidate: %w", err)
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

func (p PT700) printPage(width MediaWidth, pos pagePos, img image.PalettedImage) error {
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
	// Number of raster lines (length of the label / width of the image).
	info = binary.LittleEndian.AppendUint32(info, uint32(img.Bounds().Dx()))
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
		// Keep chain-printing enabled.
		0x00,
	}); err != nil {
		return fmt.Errorf("advanced mode settings: %w", err)
	}

	// Margin.
	if err := p.write([]byte{
		0x1B, 0x69, 0x64,
		// 1mm margins (7 dots).
		0x07, 0x00,
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
	if err := p.printRaster(img); err != nil {
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

	// Read status x2? Or maybe x3?
	s, err := p.readStatus()
	if err != nil {
		return err
	}
	fmt.Println(s)

	return nil
}

func (p PT700) printRaster(img image.PalettedImage) error {
	// Without compression no need to specify data for unused pins
	// but I think we need to specify the width offset?
	for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
		if err := p.printLine(img, x); err != nil {
			return err
		}
	}

	return nil
}

func (p PT700) printLine(img image.PalettedImage, x int) error {
	black := uint8(0)
	if img.ColorModel().(color.Palette)[0] == color.White {
		black = 1
	}

	line := make([]byte, 16)

	px := 0

	// Add the width offset.
	px += 48 // Actually this is the margin!

	// Send the whole vertical line.
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		byt := px / 8
		bit := px % 8

		if img.ColorIndexAt(x, y) == black {
			line[byt] = line[byt] | (1 << bit)
		}

		px++
	}

	return p.write(append([]byte{0x67, 16, 0}, line...))
}

func (p PT700) Status() (Status, error) {
	if err := p.write([]byte{0x1B, 0x69, 0x53}); err != nil {
		return Status{}, fmt.Errorf("status write: %w", err)
	}

	return p.readStatus()
}

func (p PT700) readStatus() (Status, error) {
	resp := make([]byte, 32)
	if err := p.read(resp, time.Second*10); err != nil {
		return Status{}, fmt.Errorf("status read: %w", err)
	}

	return Status{
		Err1:  Error1(resp[8]),
		Err2:  Error2(resp[9]),
		Width: MediaWidth(resp[10]),
		Type:  MediaType(resp[11]),
	}, nil
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

		// Always read until EOF, to make sure we don't leave any garbage behind
		// that would mess up future commands.
		n, err = readUntilEOF(int(p), buf[read:])
		if err != nil {
			return err
		}
		read += n
	}

	return nil
}

// readUntilEOF reads len(b) bytes from fd, and ensures there is nothing else immediately available to read.
// This helps catch issues if the printer returns more data than we expect, which we could try
// to parse later on.
func readUntilEOF(fd int, b []byte) (int, error) {
	read := 0

	for first := true; read != len(b); first = false {
		_ = first

		n, err := unix.Read(fd, b[read:])
		switch {
		case errors.Is(unix.EINTR, err):
			continue
		case err != nil:
			return 0, err
		//case n == 0 && first:
		// I don't understand why, but ocasionally the first read() will return 0.
		// Subsequent read()s succeed as expected, so we tolerate this.
		case n == 0:
			//return read, io.ErrUnexpectedEOF
		}

		read += n
	}

	// Make sure we're at EOF.
	/*for {
		trailing := make([]byte, 128)
		n, err := unix.Read(fd, trailing)
		switch {
		case errors.Is(unix.EINTR, err):
			continue
		case err != nil:
			return 0, err
		case n != 0:
			return read, fmt.Errorf("unexpected trailing data %v", trailing[:n])
		}

		return read, nil
	}*/

	return read, nil
}

func (p PT700) Close() error {
	return unix.Close(int(p))
}
