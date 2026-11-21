package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tungo/internal/core"
	"tungo/internal/i18n"
	"tungo/internal/proxy"
)

var pingURLs = []string{
	"https://www.google.com/generate_204",
	"https://www.youtube.com/generate_204",
	"https://www.googlevideo.com/generate_204",
	"https://www.gstatic.com/generate_204",
	"https://www.apple.com/library/test/success.html",
	"http://www.msftconnecttest.com/connecttest.txt",
}

const speedURL = "https://speed.cloudflare.com/__down?bytes=10000000"

var measureSlots = make(chan struct{}, 2)

func measure(ctx context.Context, kind core.Kind, node core.Node, speed bool) (int, float64, error) {
	if err := ctx.Err(); err != nil {
		return -1, 0, err
	}
	select {
	case measureSlots <- struct{}{}:
	case <-ctx.Done():
		return -1, 0, ctx.Err()
	}
	defer func() { <-measureSlots }()
	kernel, err := startKernel(kind, node, false)
	if err != nil {
		return -1, 0, err
	}
	defer kernel.Stop()

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: (&proxy.Upstream{Addr: kernel.SocksAddr()}).DialContext,
		},
	}
	ms := -1
	for _, url := range pingURLs {
		if ctx.Err() != nil {
			return -1, 0, ctx.Err()
		}
		if ms = roundTrip(ctx, client, url); ms > 0 {
			break
		}
	}
	if ms <= 0 {
		return -1, 0, errors.New(i18n.T("没有一个是能连上的测速地址"))
	}
	if !speed {
		return ms, 0, nil
	}

	client.Timeout = time.Minute

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, speedURL, nil)
	if err != nil {
		return ms, 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return ms, 0, err
	}
	read, err := io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if err != nil {
		return ms, 0, err
	}
	seconds := time.Since(start).Seconds()
	if seconds <= 0 {
		return ms, 0, nil
	}
	return ms, float64(read) / seconds / (1 << 20), nil
}

func roundTrip(ctx context.Context, client *http.Client, url string) int {
	if _, err := getBody(ctx, client, url); err != nil {
		return -1
	}
	best := -1
	for i := 0; i < 2; i++ {
		time.Sleep(100 * time.Millisecond)
		start := time.Now()
		if _, err := getBody(ctx, client, url); err != nil {
			return -1
		}
		if elapsed := int(time.Since(start).Milliseconds()); best < 0 || elapsed < best {
			best = elapsed
		}
	}
	return best
}

func getBody(ctx context.Context, client *http.Client, url string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return io.Copy(io.Discard, resp.Body)
}

func (a *app) realPing(i int) {
	if i < 0 || i >= len(a.nodes) || a.nodes[i].Direct() {
		return
	}
	node := a.nodes[i]
	kind := core.PickKind(core.Kind(a.kind))
	resolved := a.resolved(node)
	a.logf(i18n.T("真连接延迟：%s"), node.Name)
	ctx := a.measureCtx
	go func() {
		ms, _, err := measure(ctx, kind, resolved, false)
		a.update(func() {
			at := a.indexOfNode(node)
			if at < 0 {
				return
			}
			if err != nil {
				a.ms[at] = -1
				a.rowsDirty = true
				a.logf(i18n.T("%s：%s"), node.Name, err)
				return
			}
			a.ms[at] = ms
			a.rowsDirty = true
			a.logf(i18n.T("%s：%d ms"), node.Name, ms)
		})
	}()
}

func (a *app) speedTest(i int) {
	if i < 0 || i >= len(a.nodes) || a.nodes[i].Direct() {
		return
	}
	node := a.nodes[i]
	kind := core.PickKind(core.Kind(a.kind))
	resolved := a.resolved(node)
	a.logf(i18n.T("测速：%s"), node.Name)
	ctx := a.measureCtx
	go func() {
		ms, mb, err := measure(ctx, kind, resolved, true)
		a.update(func() {
			at := a.indexOfNode(node)
			if at < 0 {
				return
			}
			if err != nil {
				a.speed[at] = 0
				a.rowsDirty = true
				a.logf(i18n.T("%s：%s"), node.Name, err)
				return
			}
			a.ms[at] = ms
			a.speed[at] = mb
			a.rowsDirty = true
			a.logf(i18n.T("%s：%d ms，%.1f MB/s"), node.Name, ms, mb)
		})
	}()
}
func (a *app) speedLabel(i int) string {
	if i < 0 || i >= len(a.speed) || a.speed[i] <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f MB/s", a.speed[i])
}

var unlockSites = []struct {
	name string
	url  string
}{
	{"Netflix", "https://www.netflix.com/title/81215567"},
	{"YouTube Premium", "https://www.youtube.com/premium"},
	{"Disney+", "https://www.disneyplus.com/"},
	{"ChatGPT", "https://chatgpt.com/cdn-cgi/trace"},
	{"Spotify", "https://open.spotify.com/"},
}

var blockedMarkers = []string{
	"not available in your region",
	"not available in your country",
	"unavailable in your country",
	"unsupported_country",
}

func (a *app) checkUnlock() {
	addr := a.localAddr()
	if addr == "" {
		a.hint(i18n.T("先连接再检测"))
		return
	}
	a.event("%s", i18n.T("开始检测流媒体解锁"))
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{DialContext: (&proxy.Upstream{Addr: addr}).DialContext},
	}
	go func() {
		if region, err := exitRegion(client); err == nil {
			a.update(func() { a.logf(i18n.T("出口地区：%s"), region) })
		}
		for _, site := range unlockSites {
			verdict := probeSite(client, site.url)
			a.update(func() { a.logf("%s：%s", site.name, verdict) })
		}
	}()
}

func exitRegion(client *http.Client) (string, error) {
	resp, err := client.Get("https://www.cloudflare.com/cdn-cgi/trace")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(body), "\n") {
		if region, ok := strings.CutPrefix(line, "loc="); ok {
			return region, nil
		}
	}
	return "", errors.New(i18n.T("没读到出口地区"))
}

func probeSite(client *http.Client, url string) string {
	resp, err := client.Get(url)
	if err != nil {
		return i18n.T("连不上")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return i18n.T("连不上")
	}
	lower := strings.ToLower(string(body))
	for _, marker := range blockedMarkers {
		if strings.Contains(lower, marker) {
			return i18n.T("被限制")
		}
	}
	if resp.StatusCode >= 400 {
		return fmt.Sprint(resp.StatusCode)
	}
	return i18n.T("可用")
}
