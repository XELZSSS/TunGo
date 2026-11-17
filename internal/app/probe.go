package app

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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

func (a *app) measure(node core.Node, speed bool) (int, float64, error) {
	socks, err := freeAddr()
	if err != nil {
		return -1, 0, err
	}
	kernel, err := core.Start(core.PickKind(core.Kind(a.kind), node), node, socks, false)
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
		if ms = roundTrip(client, url); ms > 0 {
			break
		}
	}
	if ms <= 0 {
		return -1, 0, errors.New("没有一个是能连上的测速地址")
	}
	if !speed {
		return ms, 0, nil
	}

	client.Timeout = time.Minute

	start := time.Now()
	resp, err := client.Get(speedURL)
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

func roundTrip(client *http.Client, url string) int {
	if _, err := fetch(client, url); err != nil {
		return -1
	}
	best := -1
	for i := 0; i < 2; i++ {
		time.Sleep(100 * time.Millisecond)
		start := time.Now()
		if _, err := fetch(client, url); err != nil {
			return -1
		}
		if elapsed := int(time.Since(start).Milliseconds()); best < 0 || elapsed < best {
			best = elapsed
		}
	}
	return best
}

func fetch(client *http.Client, url string) (int64, error) {
	resp, err := client.Get(url)
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
	node := a.resolved(a.nodes[i])
	a.logf(i18n.T("真连接延迟：%s"), node.Name)
	go func() {
		ms, _, err := a.measure(node, false)
		a.update(func() {
			if i >= len(a.ms) {
				return
			}
			if err != nil {
				a.ms[i] = -1
				a.logf(i18n.T("%s：%s"), node.Name, err)
				return
			}
			a.ms[i] = ms
			a.logf(i18n.T("%s：%d ms"), node.Name, ms)
		})
	}()
}

func (a *app) speedTest(i int) {
	if i < 0 || i >= len(a.nodes) || a.nodes[i].Direct() {
		return
	}
	node := a.resolved(a.nodes[i])
	a.logf(i18n.T("测速：%s"), node.Name)
	go func() {
		ms, mb, err := a.measure(node, true)
		a.update(func() {
			if i >= len(a.ms) {
				return
			}
			if err != nil {
				a.speed[i] = 0
				a.logf(i18n.T("%s：%s"), node.Name, err)
				return
			}
			a.ms[i] = ms
			a.speed[i] = mb
			a.logf(i18n.T("%s：%d ms，%.1f MB/s"), node.Name, ms, mb)
		})
	}()
}

func freeAddr() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return l.Addr().String(), nil
}

func (a *app) speedLabel(i int) string {
	if i < 0 || i >= len(a.speed) || a.speed[i] <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f MB/s", a.speed[i])
}
