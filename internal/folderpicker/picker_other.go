//go:build !darwin && !linux && !windows

package folderpicker

import (
	"context"
	"errors"
)

func Select(context.Context) (string, error) {
	return "", errors.New("native folder selection is not supported on this platform")
}

func SelectExecutable(context.Context) (string, error) {
	return "", errors.New("native file selection is not supported on this platform")
}
