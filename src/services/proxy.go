package services

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

type Proxy struct{}

func (f *Proxy) ProxyRequest(w http.ResponseWriter, r *http.Request, target string) {
	client := &http.Client{}
	url := target + r.URL.Path
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}

	var body io.Reader
	if r.Body != nil {
		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		body = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(r.Method, url, body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	for name, values := range r.Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to make request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	if resp.Header.Get("Content-Type") == "application/json" &&
		r.URL.Path != "" &&
		!(strings.HasPrefix(r.URL.Path, "/fortnite/api/matchmaking/session/") &&
			strings.HasSuffix(r.URL.Path, "join")) {

		w.Header().Set("Content-Type", "application/json")
	}

	w.WriteHeader(resp.StatusCode)

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response body", http.StatusInternalServerError)
		return
	}

	if resp.Header.Get("Content-Type") == "application/json" {
		w.Write(bodyBytes)
		return
	}

	w.Write(bodyBytes)
}
