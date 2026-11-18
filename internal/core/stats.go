package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tungo/internal/i18n"
)

type Conn struct {
	ID      string
	Network string
	Type    string
	Host    string
	Chain   string
	Started time.Time
}

func (k *Kernel) CloseConnections() error {
	addr := k.clashAddr()
	if addr == "" {
		return fmt.Errorf(i18n.T("%s 不提供会话列表"), k.kind)
	}
	req, err := http.NewRequest(http.MethodDelete, "http://"+addr+"/connections", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+k.secret)
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("clash api：%s", resp.Status)
	}
	return nil
}

func (k *Kernel) Sessions() (up, down int64, list []Conn, err error) {
	addr := k.clashAddr()
	if addr == "" {
		return 0, 0, nil, fmt.Errorf(i18n.T("%s 不提供会话列表"), k.kind)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "http://"+addr+"/connections", nil)
	if err != nil {
		return 0, 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+k.secret)
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, nil, fmt.Errorf("clash api：%s", resp.Status)
	}
	var body struct {
		Upload   int64 `json:"uploadTotal"`
		Download int64 `json:"downloadTotal"`
		List     []struct {
			ID       string `json:"id"`
			Metadata struct {
				Network string `json:"network"`
				Type    string `json:"type"`
				Host    string `json:"host"`
				IP      string `json:"destinationIP"`
			} `json:"metadata"`
			Upload   int64    `json:"upload"`
			Download int64    `json:"download"`
			Start    string   `json:"start"`
			Chains   []string `json:"chains"`
		} `json:"connections"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, 0, nil, err
	}
	list = make([]Conn, 0, len(body.List))
	for _, one := range body.List {
		host := one.Metadata.Host
		if host == "" {
			host = one.Metadata.IP
		}
		started, _ := time.Parse(time.RFC3339, one.Start)
		list = append(list, Conn{
			ID:      one.ID,
			Network: one.Metadata.Network,
			Type:    one.Metadata.Type,
			Host:    host,
			Chain:   strings.Join(one.Chains, " → "),
			Started: started,
		})
	}
	return body.Upload, body.Download, list, nil
}
