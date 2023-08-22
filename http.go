package go_splunk_rest

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	log "log/slog"
)

func (c Connection) httpCall(method, endpoint string, headers map[string]string, data []byte) ([]byte, int, error) {
	log.Debug("httpCall",
		"method", method,
		"endpoint", endpoint,
		"headers", headers,
		"data", data)

	url := fmt.Sprintf("%s%s", c.Host, endpoint)

	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return []byte(""), 0, err
	}

	// Wrap Auth based on Connection Authentication Type
	err = c.wrapAuth(req)
	if err != nil {
		return []byte(""), 0, err
	}

	// Set Headers
	for h, v := range headers {
		req.Header.Set(h, v)
	}

	client := buildHttpClient()

	resp, err := client.Do(req)
	if err != nil {
		return []byte(""), 0, err
	}
	defer resp.Body.Close()

	respStr, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte(""), 0, err
	}

	return respStr, resp.StatusCode, nil
}

func buildHttpClient() *http.Client {
	netTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   90 * time.Second,
			KeepAlive: 60 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 30 * time.Second,
		// 	TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // uncomment line to disable TLS verification (Not Recommended)
	}
	client := &http.Client{
		Timeout:   time.Second * 90,
		Transport: netTransport,
	}

	return client
}
