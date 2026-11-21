package core

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"tungo/internal/i18n"
)

const (
	wintunURL    = "https://www.wintun.net/builds/wintun-0.14.1.zip"
	wintunSHA256 = "07c256185d6ee3652e09fa55c0b673e2624b565e02c4b9091c79ca7d2f24ef51"
)

func EnsureWintun(kernel string) error {
	path := filepath.Join(filepath.Dir(kernel), "wintun.dll")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	var archive bytes.Buffer
	if err := fetch(wintunURL, &archive, nil, 32<<20); err != nil {
		return fmt.Errorf(i18n.T("下载 wintun：%w"), err)
	}
	sum := sha256.Sum256(archive.Bytes())
	if hex.EncodeToString(sum[:]) != wintunSHA256 {
		return errors.New(i18n.T("wintun 包的校验不通过"))
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
