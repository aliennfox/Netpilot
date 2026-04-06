package monitor

import (
	"syscall"
	"unsafe"
)

type termios struct {
	Iflag  uint64
	Oflag  uint64
	Cflag  uint64
	Lflag  uint64
	Cc     [20]uint8
	Ispeed uint64
	Ospeed uint64
}

const (
	ioctlGetTermios = syscall.TIOCGETA
	ioctlSetTermios = syscall.TIOCSETA
)

// makeRaw 将终端设置为 raw 模式（不回显、不缓冲）
func makeRaw(fd uintptr) (*termios, error) {
	var old termios
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, ioctlGetTermios, uintptr(unsafe.Pointer(&old)), 0, 0, 0); err != 0 {
		return nil, err
	}

	raw := old
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, ioctlSetTermios, uintptr(unsafe.Pointer(&raw)), 0, 0, 0); err != 0 {
		return nil, err
	}
	return &old, nil
}

// restore 恢复终端设置
func restore(fd uintptr, old *termios) {
	syscall.Syscall6(syscall.SYS_IOCTL, fd, ioctlSetTermios, uintptr(unsafe.Pointer(old)), 0, 0, 0) //nolint:errcheck
}
