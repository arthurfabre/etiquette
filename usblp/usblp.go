package usblp

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

type ID struct {
	VendorID  uint16
	ProductID uint16
}

type Device struct {
	path string
	fd   int
}

func Connected() ([]*Device, error) {
	lps, err := fs.Glob(os.DirFS("/dev/usb"), "lp[0-9]*")
	if err != nil {
		return nil, err
	}

	var devices []*Device
	for _, lp := range lps {
		dev, err := Open(path.Join("/dev/usb", lp))
		if err != nil {
			return nil, err
		}

		devices = append(devices, dev)
	}

	return devices, nil
}

func Open(path string) (*Device, error) {
	fd, err := unix.Open(path, unix.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	return &Device{
		path: path,
		fd:   fd,
	}, nil
}

func (d *Device) Path() string {
	return d.path
}

func (d *Device) ID() (ID, error) {
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

	// This IOCTL is an platform dependent mess. It expects [2]int, but assumes int == uint16.
	//
	var id struct {
		vendorID  uint16
		_         uint16
		productID uint16
		_         uint16
	}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(d.fd), uintptr(
		IOCNR_GET_VID_PID<<IOC_NRSHIFT|
			'P'<<IOC_TYPESHIFT|
			unsafe.Sizeof(id)<<IOC_SIZESHIFT|
			IOC_READ<<IOC_DIRSHIFT,
	), uintptr(unsafe.Pointer(&id)))
	if errno != 0 {
		return ID{}, fmt.Errorf("GET_VID_PID: %w", errno)
	}

	return ID{
		VendorID:  id.vendorID,
		ProductID: id.productID,
	}, nil
}

func (d *Device) Write(b []byte) error {
	for wrote := 0; wrote != len(b); {
		n, err := unix.Write(int(d.fd), b[wrote:])
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
func (d *Device) Read(buf []byte, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	pollFds := []unix.PollFd{
		{Fd: int32(d.fd), Events: unix.POLLIN},
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

		n, err = readFull(int(d.fd), buf[read:])
		if err != nil {
			return err
		}
		read += n
	}

	return nil
}

func (d *Device) Discard() error {
	return readUntilEOF(d.fd)
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

func (d *Device) Close() error {
	return unix.Close(int(d.fd))
}
