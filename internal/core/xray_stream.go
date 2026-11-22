package core

func xrayStream(n Node) map[string]any {
	stream := map[string]any{}
	switch SecurityOf(n) {
	case "tls":
		tls := map[string]any{"serverName": serverName(n)}
		if alpn := alpnOf(n); len(alpn) > 0 {
			tls["alpn"] = alpn
		}
		if n.Insecure {
			tls["allowInsecure"] = true
		}
		if n.ECH != "" {
			tls["echConfigList"] = n.ECH
		}
		if n.Pinned != "" {
			tls["pinnedPeerCertSha256"] = n.Pinned
		}
		if n.Fingerprint != "" {
			tls["fingerprint"] = n.Fingerprint
		}
		stream["security"] = "tls"
		stream["tlsSettings"] = tls
	case "reality":
		reality := map[string]any{
			"serverName": serverName(n),
			"publicKey":  n.PublicKey,
			"shortId":    n.ShortID,
		}
		if n.Fingerprint != "" {
			reality["fingerprint"] = n.Fingerprint
		}
		stream["security"] = "reality"
		stream["realitySettings"] = reality
	}
	switch n.Network {
	case "ws":
		ws := map[string]any{"path": wsPath(n)}
		if path, early := earlyData(wsPath(n)); early > 0 {
			ws["path"] = path
			ws["maxEarlyData"] = early
			ws["earlyDataHeaderName"] = "Sec-WebSocket-Protocol"
		}
		if n.Host != "" {
			ws["headers"] = map[string]any{"Host": n.Host}
		}
		stream["network"] = "ws"
		stream["wsSettings"] = ws
	case "grpc":
		stream["network"] = "grpc"
		stream["grpcSettings"] = map[string]any{"serviceName": n.Path}
	case "http":

		stream["network"] = "xhttp"
		stream["xhttpSettings"] = map[string]any{"host": hostName(n), "path": wsPath(n)}
	case "httpupgrade":
		upgrade := map[string]any{"path": wsPath(n)}
		if n.Host != "" {
			upgrade["host"] = n.Host
		}
		stream["network"] = "httpupgrade"
		stream["httpupgradeSettings"] = upgrade
	}
	return stream
}
