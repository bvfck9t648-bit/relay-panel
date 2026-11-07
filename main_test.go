package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTargetPolicy(t *testing.T) {
	for _, s := range []string{"http://127.0.0.1:9081", "https://[::1]:8443", "http://localhost:3000"} {
		if _, e := validTarget(s); e != nil {
			t.Errorf("expected valid %s: %v", s, e)
		}
	}
	for _, s := range []string{"file:///etc/passwd", "http://example.com", "http://169.254.169.254", "http://127.0.0.1:9090", "http://user:pass@localhost:3000"} {
		if _, e := validTarget(s); e == nil {
			t.Errorf("expected rejection %s", s)
		}
	}
}
func TestProxyPrivacy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("sensitive headers forwarded")
		}
		if r.URL.Path != "/login" {
			t.Error("path changed")
		}
		w.Header().Set("Set-Cookie", "secret=hidden")
		w.WriteHeader(201)
		io.WriteString(w, "demo")
	}))
	defer upstream.Close()
	cfg.Target = upstream.URL
	req := httptest.NewRequest("GET", "https://hiddencontact.dpdns.org/login?secret=notlogged", nil)
	req.Header.Set("Authorization", "secret")
	req.Header.Set("Cookie", "secret")
	w := httptest.NewRecorder()
	serveProxy(w, req)
	if w.Code != 201 || strings.TrimSpace(w.Body.String()) != "demo" {
		t.Fatalf("bad response %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Set-Cookie") != "" {
		t.Error("cookie leaked")
	}
	if events[0].Path != "/login" {
		t.Error("query logged")
	}
}
