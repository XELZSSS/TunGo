package core

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"tungo/internal/i18n"
)

const wintunURL = "https://www.wintun.net/builds/wintun-0.14.1.zip"

func EnsureWintun(kernel string) error {
	path := filepath.Join(filepath.Dir(kernel), "wintun.dll")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	var archive bytes.Buffer
	if err := fetch(wintunURL, &archive, nil, 32<<20); err != nil {
		return fmt.Errorf(i18n.T("下载 wintun：%w"), err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		return fmt.Errorf(i18n.T("wintun 包打不开：%w"), err)
	}
	want := "bin/" + wintunArch() + "/wintun.dll"
	for _, file := range reader.File {
		if !strings.HasSuffix(file.Name, want) {
			continue
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		defer in.Close()
		return write(in, path)
	}
	return fmt.Errorf(i18n.T("wintun 包里没有 %s"), want)
}

func wintunArch() string {
	if runtime.GOARCH == "386" {
		return "x86"
	}
	return runtime.GOARCH
}
