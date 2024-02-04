package pt700

import (
	"errors"
	"fmt"
	"io"
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

func (p PT700) Invalidate() error {
	// Invalidate. Brother docs 2.1.1 sends 100 bytes, so we do too.
	if err := p.write(make([]byte, 100)); err != nil {
		return fmt.Errorf("invalidate: %w", err)
	}
	return nil
}

func initialize(printer *os.File) error {
	// TODO - this seemingly starts printing, and then we can't send status more than once?
	// Query the status first, and then start printing.
	// Initialize.
	/*_, err = printer.Write([]byte{0x1B, 0x40})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}*/

	return nil
}

func (p PT700) Status() (Status, error) {
	if err := p.write([]byte{0x1B, 0x69, 0x53}); err != nil {
		return Status{}, fmt.Errorf("status write: %w", err)
	}

	resp := make([]byte, 32)
	if err := p.read(resp, time.Second); err != nil {
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
		n, err := unix.Read(fd, b[read:])
		switch {
		case errors.Is(unix.EINTR, err):
			continue
		case err != nil:
			return 0, err
		case n == 0 && first:
			// I don't understand why, but ocasionally the first read() will return 0.
			// Subsequent read()s succeed as expected, so we tolerate this.
		case n == 0:
			return read, io.ErrUnexpectedEOF
		}

		read += n
	}

	// Make sure we're at EOF.
	for {
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
	}
}

func (p PT700) Close() error {
	return unix.Close(int(p))
}
