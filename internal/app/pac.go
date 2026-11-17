package app

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	"tungo/internal/i18n"
)

const pacScript = `function FindProxyForURL(url, host) {
  if (isPlainHostName(host) ||
      shExpMatch(host, "*.local") ||
      isInNet(host, "127.0.0.0", "255.0.0.0") ||
      isInNet(host, "10.0.0.0", "255.0.0.0") ||
      isInNet(host, "172.16.0.0", "255.240.0.0") ||
      isInNet(host, "192.168.0.0", "255.255.0.0") ||
      isInNet(host, "169.254.0.0", "255.255.0.0")) {
    return "DIRECT";
  }
  return "PROXY %s;DIRECT;";
}`

func (a *app) startPac() (string, error) {
	a.stopPac()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		fmt.Fprintf(w, pacScript, a.localAddr())
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("%s: %w", i18n.T("PAC 端口起不来"), err)
	}
	server := &http.Server{Handler: handler}
	a.pac = server
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.update(func() { a.logf("PAC: %s", err) })
		}
	}()
	return "http://" + listener.Addr().String() + "/pac", nil
}

func (a *app) stopPac() {
	if a.pac != nil {
		a.pac.Close()
		a.pac = nil
	}
}
