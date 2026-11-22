package core

func singBoxTLS(n Node) map[string]any {
	tls := map[string]any{"enabled": true, "server_name": serverName(n)}
	if n.Insecure {
		tls["insecure"] = true
	}
	if alpn := alpnOf(n); len(alpn) > 0 {
		tls["alpn"] = alpn
	}
	if n.ECH != "" {
		tls["ech"] = singBoxECH(n.ECH)
	}
	fingerprint := n.Fingerprint
	if SecurityOf(n) == "reality" && fingerprint == "" {
		fingerprint = "chrome"
	}
	if fingerprint != "" {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": fingerprint}
	}
	quic := n.Protocol == "hysteria2" || n.Protocol == "tuic"
	if n.Fragment && !quic {
		tls["fragment"] = true
	}
	if n.RecordFragment && !quic {
		tls["record_fragment"] = true
	}
	if SecurityOf(n) == "reality" {
		tls["reality"] = map[string]any{
			"enabled":    true,
			"public_key": n.PublicKey,
			"short_id":   n.ShortID,
		}
	}
	return tls
}

func singBoxTransport(n Node) map[string]any {
	switch n.Network {
	case "ws":

		path, early := earlyData(wsPath(n))
		ws := map[string]any{"type": "ws", "path": path}
		if early > 0 {
			ws["max_early_data"] = early
			ws["early_data_header_name"] = "Sec-WebSocket-Protocol"
		}
		if n.Host != "" {
			ws["headers"] = map[string]any{"Host": n.Host}
		}
		return ws
	case "grpc":
		return map[string]any{"type": "grpc", "service_name": n.Path}
	case "http":
		return map[string]any{"type": "http", "host": []string{hostName(n)}, "path": wsPath(n)}
	case "httpupgrade":
		upgrade := map[string]any{"type": "httpupgrade", "path": wsPath(n)}
		if n.Host != "" {
			upgrade["host"] = n.Host
		}
		return upgrade
	}
	return nil
}
