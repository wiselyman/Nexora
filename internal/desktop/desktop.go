package desktop

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
)

var errUnsupported = errors.New("desktop control is not supported on this platform")

type Adapter interface {
	Info() map[string]any
	Screenshot() ([]byte, error)
	Windows() ([]map[string]any, error)
	Pointer(x, y, button int) error
	Key(keys string) error
	Type(text string) error
}

const maxScreenshotBytes = 32 << 20
const maxScreenshotDimension = 16384
const maxScreenshotPixels = 32 << 20

// screenshotPath creates a private, unique temporary directory so concurrent
// screenshots cannot overwrite or read one another's capture.
func screenshotPath() (string, func(), error) {
	dir, err := os.MkdirTemp("", "5060-screen-*")
	if err != nil {
		return "", nil, err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	return filepath.Join(dir, "screen.png"), func() { _ = os.RemoveAll(dir) }, nil
}

func readScreenshot(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("截图结果不是普通文件")
	}
	if info.Size() <= 0 || info.Size() > maxScreenshotBytes {
		return nil, fmt.Errorf("截图大小超出限制（最大 %d MiB）", maxScreenshotBytes>>20)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("系统返回的截图不是有效 PNG 图片")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxScreenshotDimension || config.Height > maxScreenshotDimension || int64(config.Width)*int64(config.Height) > maxScreenshotPixels {
		return nil, fmt.Errorf("截图尺寸 %d×%d 超出限制（每边最大 %d 像素，最多 %d 百万像素）", config.Width, config.Height, maxScreenshotDimension, maxScreenshotPixels>>20)
	}
	return data, nil
}
