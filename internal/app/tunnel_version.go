package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const minimumTunnelClientVersion = "0.0.17"

var tunnelVersionPattern = regexp.MustCompile(`(?m)(?:^|[^0-9A-Za-z])v?(\d+)\.(\d+)\.(\d+)(?:\+[0-9A-Za-z.-]+)?(?:$|[^0-9A-Za-z.+-])`)

type versionOutput struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	cancel   context.CancelFunc
	tooLarge bool
}

func (v *versionOutput) Write(p []byte) (int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	const maxVersionOutput = 4 << 10
	remaining := maxVersionOutput - v.buffer.Len()
	if remaining > 0 {
		if len(p) > remaining {
			_, _ = v.buffer.Write(p[:remaining])
		} else {
			_, _ = v.buffer.Write(p)
		}
	}
	if len(p) > remaining && !v.tooLarge {
		v.tooLarge = true
		v.cancel()
	}
	return len(p), nil
}

func inspectTunnelClient(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output := &versionOutput{cancel: cancel}
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	output.mu.Lock()
	data := append([]byte(nil), output.buffer.Bytes()...)
	tooLarge := output.tooLarge
	output.mu.Unlock()
	if tooLarge {
		return "", errors.New("tunnel-client 版本命令输出过长，请确认路径指向 OpenAI 官方客户端")
	}
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", errors.New("读取 tunnel-client 版本超时，请确认路径指向官方客户端")
		}
		return "", errors.New("无法读取 tunnel-client 版本，请安装 OpenAI 官方客户端")
	}
	match := tunnelVersionPattern.FindSubmatch(data)
	if len(match) != 4 {
		return "", errors.New("无法识别 tunnel-client 版本；请安装 OpenAI 官方发布的客户端")
	}
	major, err1 := strconv.Atoi(string(match[1]))
	minor, err2 := strconv.Atoi(string(match[2]))
	patch, err3 := strconv.Atoi(string(match[3]))
	if err1 != nil || err2 != nil || err3 != nil {
		return "", errors.New("无法识别 tunnel-client 版本；请安装 OpenAI 官方发布的客户端")
	}
	version := fmt.Sprintf("%d.%d.%d", major, minor, patch)
	if major == 0 && minor == 0 && patch < 17 {
		return version, fmt.Errorf("tunnel-client %s 版本过旧；Nexora 需要 v%s 或更高版本，请更新官方客户端", version, minimumTunnelClientVersion)
	}
	return version, nil
}
