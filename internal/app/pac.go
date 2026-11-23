package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"tungo/internal/core"
	"tungo/internal/i18n"
)

const stopTimeout = 2 * time.Second

func (a *app) startPac(addr string) (string, error) {
	a.stopPac()
	script := core.PacScript(addr, core.LoadRouting())
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		fmt.Fprint(w, script)
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
		ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
		a.pac.Shutdown(ctx)
		cancel()
		a.pac = nil
	}
}
