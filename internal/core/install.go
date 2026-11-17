package core

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"tungo/internal/i18n"
)

const (
	githubURL = "https://github.com/"
	githubAPI = "https://api.github.com/repos/"
)

type release struct {
	repo string
	tag  string
}

var releases = map[Kind]release{
	SingBox: {repo: "SagerNet/sing-box", tag: "v1.14.4"},
	Xray:    {repo: "XTLS/Xray-core", tag: "v26.10.31"},
}

func Latest(kind Kind) (string, error) {
	url := githubURL + releases[kind].repo + "/releases/latest"
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: transport(),
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "TunGo")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	parts := strings.Split(resp.Header.Get("Location"), "/tag/")
	if len(parts) < 2 || parts[1] == "" {
		return "", fmt.Errorf(i18n.T("%s：没有版本号"), url)
	}
	return parts[1], nil
}

func Version(kind Kind) string {
	binary, err := Binary(kind)
	if err != nil {
		return ""
	}
	out, err := versionOf(kind, binary)
	if err != nil {
		return ""
	}
	return numbers(string(out))
}

func versionOf(kind Kind, binary string) ([]byte, error) {
	args := []string{"version"}
	if kind == Xray {
		args = []string{"-version"}
	}
	cmd := exec.Command(binary, args...)
	prepare(cmd)
	return cmd.CombinedOutput()
}

func numbers(text string) string {
	for _, field := range strings.Fields(text) {
		field = strings.Trim(field, "v()[],")
		if len(fields(field)) >= 2 {
			return field
		}
	}
	return ""
}

func Newer(tag, version string) bool {
	a, b := fields(tag), fields(version)
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	for i := 0; i < len(a) || i < len(b); i++ {
		x, y := 0, 0
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func fields(version string) []int {
	version = strings.TrimPrefix(version, "v")
	var out []int
	for _, part := range strings.Split(version, ".") {
		number, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		out = append(out, number)
	}
	return out
}

func Install(kind Kind, tag string, progress func(part, whole int64)) (string, error) {
	if tag == "" {
		var err error
		if tag, err = Latest(kind); err != nil {
			return "", err
		}
	}
	archive, err := file(kind, tag)
	if err != nil {
		return "", err
	}
	url := githubURL + releases[kind].repo + "/releases/download/" + tag + "/" + archive
	want := assetDigest(kind, tag, archive)
	if want == "" {
		log.Print("no digest for ", archive)
	}

	temp, err := os.CreateTemp("", "tungo-"+string(kind)+"-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()

	if err := fetch(url, temp, progress); err != nil {
		return "", err
	}
	if ok, err := looks(archive, temp); err != nil {
		return "", err
	} else if !ok {
		return "", fmt.Errorf(i18n.T("%s：返回的不是安装包"), url)
	}
	if err := verify(want, url, temp); err != nil {
		return "", err
	}

	bin, err := binDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return "", err
	}
	name := string(kind)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(bin, name)
	fresh := target + ".new"
	os.Remove(fresh)
	if err := unpack(temp.Name(), archive, string(kind), fresh); err != nil {
		os.Remove(fresh)
		return "", err
	}
	if err := runs(kind, fresh); err != nil {
		os.Remove(fresh)
		return "", err
	}
	if err := os.Rename(fresh, target); err != nil {
		os.Remove(fresh)
		return "", err
	}
	return target, nil
}

func file(kind Kind, tag string) (string, error) {
	platform, err := platform(kind)
	if err != nil {
		return "", err
	}
	if kind == Xray {
		return "Xray-" + platform + ".zip", nil
	}
	version := strings.TrimPrefix(tag, "v")
	if runtime.GOOS == "windows" {
		return "sing-box-" + version + "-" + platform + ".zip", nil
	}
	return "sing-box-" + version + "-" + platform + ".tar.gz", nil
}

func platform(kind Kind) (string, error) {
	system := runtime.GOOS + "/" + runtime.GOARCH
	if kind == SingBox {
		return runtime.GOOS + "-" + runtime.GOARCH, nil
	}
	names := map[string]string{
		"linux/amd64":   "linux-64",
		"linux/386":     "linux-32",
		"linux/arm64":   "linux-arm64-v8a",
		"linux/arm":     "linux-arm32-v7a",
		"windows/amd64": "windows-64",
		"windows/386":   "windows-32",
		"windows/arm64": "windows-arm64-v8a",
		"darwin/amd64":  "macos-64",
		"darwin/arm64":  "macos-arm64-v8a",
	}
	name, ok := names[system]
	if !ok {
		return "", fmt.Errorf(i18n.T("%s 没有为 %s 发布的包"), kind, system)
	}
	return name, nil
}

func assetDigest(kind Kind, tag, name string) string {
	url := githubAPI + releases[kind].repo + "/releases/tags/" + tag
	var body bytes.Buffer
	if err := fetch(url, &body, nil); err != nil {
		return ""
	}
	var got struct {
		Assets []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body.Bytes(), &got); err != nil {
		return ""
	}
	for _, asset := range got.Assets {
		if asset.Name == name {
			return strings.TrimPrefix(asset.Digest, "sha256:")
		}
	}
	return ""
}

func verify(want, url string, from *os.File) error {
	if want == "" {
		return nil
	}
	if _, err := from.Seek(0, io.SeekStart); err != nil {
		return err
	}
	sum := sha256.New()
	if _, err := io.Copy(sum, from); err != nil {
		return err
	}
	if got := hex.EncodeToString(sum.Sum(nil)); got != want {
		return fmt.Errorf(i18n.T("%s：SHA256 对不上（%s）"), url, got)
	}
	return nil
}

func looks(archive string, from *os.File) (bool, error) {
	if _, err := from.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	var head [4]byte
	n, err := io.ReadFull(from, head[:])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return false, err
	}
	if strings.HasSuffix(archive, ".zip") {
		return n == len(head) && string(head[:]) == "PK\x03\x04", nil
	}
	return n >= 2 && head[0] == 0x1f && head[1] == 0x8b, nil
}

func fetch(url string, to io.Writer, progress func(part, whole int64)) error {
	client := &http.Client{Timeout: 10 * time.Minute, Transport: transport()}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "TunGo")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("%s：%s", url, resp.Status)
	}
	var body io.Reader = resp.Body
	if progress != nil {
		body = &progressReader{from: resp.Body, whole: resp.ContentLength, report: progress}
	}
	if _, err := io.Copy(to, body); err != nil {
		return fmt.Errorf("%s：%w", url, err)
	}
	return nil
}

func transport() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 8 * time.Second}).DialContext,
		TLSHandshakeTimeout:   8 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
}

type progressReader struct {
	from   io.Reader
	done   int64
	whole  int64
	report func(part, whole int64)
	last   time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.from.Read(b)
	p.done += int64(n)
	if now := time.Now(); now.Sub(p.last) > 100*time.Millisecond {
		p.last = now
		p.report(p.done, p.whole)
	}
	return n, err
}

func unpack(archive, name, kind, target string) error {
	if strings.HasSuffix(name, ".zip") {
		return unzip(archive, kind, target)
	}
	return untar(archive, kind, target)
}

func unzip(path, kind, target string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if base(file.Name) != kind {
			continue
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		defer in.Close()
		return write(in, target)
	}
	return fmt.Errorf(i18n.T("发布包里没有 %s"), kind)
}

func untar(path, kind, target string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if base(header.Name) != kind {
			continue
		}
		return write(reader, target)
	}
	return fmt.Errorf(i18n.T("发布包里没有 %s"), kind)
}

func base(name string) string {
	name = filepath.Base(name)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func write(r io.Reader, target string) error {
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, r); err != nil {
		file.Close()
		os.Remove(target)
		return err
	}
	return file.Close()
}

func runs(kind Kind, binary string) error {
	if _, err := versionOf(kind, binary); err != nil {
		return fmt.Errorf(i18n.T("%s 装好后跑不起来：%w"), kind, err)
	}
	return nil
}
