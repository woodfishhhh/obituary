package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const scriptLimit = 1 << 20

func (p plan) scriptURLs(name string) []string {
	tag := version(p.version)
	asset := name
	if name == "install.ps1" {
		asset = "packaging/" + asset
	}
	github := "https://raw.githubusercontent.com/woodfishhhh/obituary/" + tag + "/" + asset
	if p.source == "mirror" {
		return []string{"https://cn.obituary.woodfish.site/download/" + tag + "/" + name, github}
	}
	return []string{github}
}

func (p plan) download(ctx context.Context, home string, log io.Writer) (string, error) {
	name := p.script()
	if name == "" {
		return "", nil
	}
	var failures error
	for _, endpoint := range p.scriptURLs(name) {
		data, err := fetchScript(ctx, endpoint)
		if err != nil {
			fmt.Fprintln(log, err)
			failures = errors.Join(failures, err)
			continue
		}
		return writeScript(home, name, data)
	}
	return "", failures
}

func fetchScript(ctx context.Context, endpoint string) ([]byte, error) {
	resp, err := (Checker{}).request(ctx, endpoint, "")
	if err != nil {
		return nil, fmt.Errorf("download installer %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		return nil, fmt.Errorf("download installer %s: HTTP %d (%s)", endpoint, resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, scriptLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read installer %s: %w", endpoint, err)
	}
	if len(data) == 0 || len(data) > scriptLimit {
		return nil, fmt.Errorf("unexpected installer size from %s", endpoint)
	}
	return data, nil
}

func writeScript(home, name string, data []byte) (string, error) {
	f, err := os.CreateTemp(home, "update-*"+filepath.Ext(name))
	if err != nil {
		return "", err
	}
	_, err = f.Write(data)
	if err := errors.Join(err, f.Close()); err != nil {
		return "", errors.Join(err, os.Remove(f.Name()))
	}
	return f.Name(), nil
}
