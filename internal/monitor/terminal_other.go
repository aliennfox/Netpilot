//go:build !darwin

package monitor

import "errors"

type termios struct{}

func makeRaw(fd uintptr) (*termios, error) {
	return nil, errors.New("terminal raw mode not supported on this platform")
}

func restore(fd uintptr, old *termios) {}
