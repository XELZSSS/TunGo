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
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"tungo/internal/appdir"
	"tungo/internal/i18n"
)

type release struct {
	repo string
	tag  string
}

var releases = map[Kind]release{
	SingBox: {repo: "SagerNet/sing-box", tag: "v1.14.4"},
	Xray:    {repo: "XTLS/Xray-core", tag: "v26.10.31"},
}

var mirrors = []string{
	"https://ghfast.top/",
	"https://ghproxy.cn/",
	"https://gh-proxy.com/",
	"",
}

var apiMirrors = []string{
	"https://gh-proxy.com/",
	"",
}

func Ensure(kind Kind, progress func(part, whole int64)) (string, error) {
	if binary, err := Binary(kind); err == nil {
		return binary, nil
	}
	tag, err := Latest(kind)
	if err != nil {

		return Install(kind, "", progress)
	}
	return Install(kind, tag, progress)
}

func Latest(kind Kind) (string, error) {
	url := "https://api.github.com/repos/" + releases[kind].repo + "/releases/latest"
	var last error
	for _, mirror := range apiMirrors {
		tag, err := tagOf(mirror + url)
		if err == nil {
			return tag, nil
		}
		last = err
	}
	return "", last
}

func tagOf(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second, Transport: transport()}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s：%s", url, resp.Status)
	}
	var release struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.Tag == "" {
		return "", fmt.Errorf(i18n.T("%s：没有版本号"), url)
	}
	return release.Tag, nil
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
		if isVersion(field) {
			return field
		}
	}
	return ""
}

func isVersion(text string) bool {
	parts := strings.Split(text, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return false
			}
		}
	}
	return true
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
	rel := releases[kind]
	if tag == "" {
		tag = rel.tag
	}
	archive, err := file(kind, tag)
	if err != nil {
		return "", err
	}
	github := "https://github.com/" + rel.repo + "/releases/download/" + tag + "/" + archive

	temp, err := os.CreateTemp("", "tungo-"+string(kind)+"-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()

	if err := download(kind, github, temp, progress); err != nil {
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

func download(kind Kind, github string, to *os.File, progress func(part, whole int64)) error {
	urls := reachable(github)
	if len(urls) == 0 {
		return fmt.Errorf(i18n.T("下载地址都连不上，可在 %s 里换成你能用的镜像"), mirrorsFile())
	}
	var last error
	for _, url := range urls {
		if err := to.Truncate(0); err != nil {
			return err
		}
		if _, err := to.Seek(0, io.SeekStart); err != nil {
			return err
		}
		err := fetch(url, to, progress)
		if err == nil {
			ok, checkErr := looks(github, to)
			if checkErr != nil {
				return checkErr
			}
			if ok {
				if err = verify(kind, github, to); err == nil {
					return nil
				}
			} else {
				err = fmt.Errorf(i18n.T("%s：返回的不是安装包"), url)
			}
		}
		last = err
	}
	return last
}

func verify(kind Kind, github string, from *os.File) error {
	if kind != Xray {
		return nil
	}
	want, err := digest(github + ".dgst")
	if err != nil {
		return err
	}
	if _, err := from.Seek(0, io.SeekStart); err != nil {
		return err
	}
	sum := sha256.New()
	if _, err := io.Copy(sum, from); err != nil {
		return err
	}
	if got := hex.EncodeToString(sum.Sum(nil)); got != want {
		return fmt.Errorf(i18n.T("%s：SHA256 对不上（%s）"), github, got)
	}
	return nil
}

func digest(url string) (string, error) {
	urls := reachable(url)
	if len(urls) == 0 {
		return "", fmt.Errorf(i18n.T("%s 连不上"), url)
	}
	var last error
	for _, one := range urls {
		var body bytes.Buffer
		if err := fetch(one, &body, nil); err != nil {
			last = err
			continue
		}
		for _, line := range strings.Split(body.String(), "\n") {
			if value, ok := strings.CutPrefix(strings.TrimSpace(line), "SHA256="); ok {
				return strings.ToLower(strings.TrimSpace(value)), nil
			}
		}
		last = fmt.Errorf(i18n.T("%s：没有 SHA256"), one)
	}
	return "", last
}

type answer struct {
	url string
	at  time.Duration
}

func reachable(github string) []string {
	answers := make(chan answer, len(mirrorList()))
	var wg sync.WaitGroup
	for _, mirror := range mirrorList() {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			start := time.Now()
			if has(url) {
				answers <- answer{url, time.Since(start)}
			}
		}(mirror + github)
	}
	go func() {
		wg.Wait()
		close(answers)
	}()

	var found []answer
	wait := time.NewTimer(10 * time.Second)
	defer wait.Stop()
	settling := false
	for {
		select {
		case a, ok := <-answers:
			if !ok {
				return order(found)
			}
			found = append(found, a)
			if !settling {
				settling = true
				wait.Reset(800 * time.Millisecond)
			}
		case <-wait.C:
			return order(found)
		}
	}
}

func order(found []answer) []string {
	sort.Slice(found, func(i, j int) bool { return found[i].at < found[j].at })
	urls := make([]string, len(found))
	for i, a := range found {
		urls[i] = a.url
	}
	return urls
}

func has(url string) bool {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Range", "bytes=0-0")
	client := &http.Client{Timeout: 8 * time.Second, Transport: transport()}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusPartialContent || resp.StatusCode == http.StatusOK
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
	resp, err := client.Get(url)
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

func WriteMirrors() error {
	path := mirrorsFile()
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(defaultMirrors()), 0o600)
}

func mirrorList() []string {
	data, err := os.ReadFile(mirrorsFile())
	if err != nil {
		return mirrors
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "github" {
			line = ""
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		return mirrors
	}
	return out
}

func defaultMirrors() string {
	lines := make([]string, 0, len(mirrors))
	for _, mirror := range mirrors {
		if mirror == "" {
			mirror = "github"
		}
		lines = append(lines, mirror)
	}
	return i18n.T("# 内核下载走哪些地址，一行一个，按顺序试。") + "\n" +
		i18n.T("# github 表示直连 GitHub（放在最后，前面都不行时才用它）。") + "\n" +
		i18n.T("# 换成你能用的镜像即可，例如公司内网的文件代理。") + "\n" +
		strings.Join(lines, "\n") + "\n"
}

func mirrorsFile() string {
	d, err := appdir.Dir()
	if err != nil {
		return "mirrors.txt"
	}
	return filepath.Join(d, "mirrors.txt")
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
