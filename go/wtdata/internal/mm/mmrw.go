package mm

import (
	"os"

	"golang.org/x/sys/unix"
)

// MapRW maps the given file for read-write access. The file must exist and have the desired size.
func MapRW(f *os.File, length int) ([]byte, error) {
	fd := int(f.Fd())
	data, err := unix.Mmap(fd, 0, length, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Unmap unmaps a previously mapped region.
func Unmap(b []byte) error {
	return unix.Munmap(b)
}

// EnsureSize grows the file to the given size if smaller.
func EnsureSize(f *os.File, size int64) error {
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.Size() >= size {
		return nil
	}
	return f.Truncate(size)
}