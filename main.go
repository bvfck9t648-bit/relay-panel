package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed web/index.html
var page []byte

type Config struct {
	Target string `json:"target"`
}
type Event struct {
	Time     string `json:"time"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Status   int    `json:"status"`
	Duration int64  `json:"duration"`
}

var mu sync.RWMutex
var cfg = Config{Target: "http://127.0.0.1:9081"}
var events = []Event{}
var count uint64
var configPath = "/var/lib/proxy-panel/config.json"

func validTarget(s string) (*url.URL, error) {
	u, e := url.Parse(s)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("Geçersiz HTTP(S) hedefi")
	}
	// Restrict network destinations: changing this requires an explicit administrator deployment decision.
	h := u.Hostname()
	ip := net.ParseIP(h)
	if h != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("Bu sürüm yalnızca localhost veya loopback IP hedeflerini kabul eder")
	}
	p := u.Port()
	if p == "9080" || p == "9090" || p == "9999" || p == "80" || p == "443" {
		return nil, fmt.Errorf("Proxy/panel portuna döngü oluşturulamaz")
	}
	return u, nil
}
func jsonReply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func main() {
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		configPath = p
	}
	if b, e := os.ReadFile(configPath); e == nil {
		if json.Unmarshal(b, &cfg) != nil {
			log.Fatal("Geçersiz yapılandırma")
		}
	}
	if _, e := validTarget(cfg.Target); e != nil {
		log.Fatal(e)
	}
	demo := http.NewServeMux()
	demo.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html lang="tr"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Yerel demo</title><body style="background:#0c1222;color:#e6edf8;font:18px system-ui;display:grid;place-items:center;min-height:95vh"><main style="max-width:580px;padding:40px;border:1px solid #293451;border-radius:24px"><small style="color:#5eeac2">● PROXY AKTİF</small><h1>Bağlantı başarılı.</h1><p>Bu sayfa VDS üzerinde çalışan yerel demo servisinden geliyor.</p><p>Gerçek bir giriş formu veya kimlik bilgisi toplama özelliği içermez.</p><a style="color:#89aaff" href="/login/">Demoyu yenile ↗</a></main></body></html>`)
	})
	go func() {
		log.Fatal((&http.Server{Addr: "127.0.0.1:9081", Handler: demo, ReadHeaderTimeout: 5 * time.Second}).ListenAndServe())
	}()
	public := http.NewServeMux()
	public.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") })
	public.HandleFunc("/login", serveProxy)
	public.HandleFunc("/login/", serveProxy)
	panel := http.NewServeMux()
	panel.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
	})
	panel.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		jsonReply(w, map[string]any{"config": cfg, "events": events, "requests": count})
	})
	panel.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "JSON gerekli", 415)
			return
		}
		var next Config
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		d.DisallowUnknownFields()
		if d.Decode(&next) != nil {
			http.Error(w, "Geçersiz yapılandırma", 400)
			return
		}
		if _, e := validTarget(next.Target); e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		b, _ := json.Marshal(next)
		if e := os.WriteFile(configPath+".tmp", b, 0600); e != nil {
			http.Error(w, "Kaydedilemedi", 500)
			return
		}
		if e := os.Rename(configPath+".tmp", configPath); e != nil {
			http.Error(w, "Kaydedilemedi", 500)
			return
		}
		cfg = next
		jsonReply(w, map[string]bool{"ok": true})
	})
	go func() {
		log.Fatal((&http.Server{Addr: "127.0.0.1:9090", Handler: secure(panel), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}).ListenAndServe())
	}()
	log.Print("Proxy 9080, panel 9090, demo 9081: yalnızca loopback")
	log.Fatal((&http.Server{Addr: "127.0.0.1:9080", Handler: secure(public), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 1 << 20}).ListenAndServe())
}
func secure(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}
func serveProxy(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	mu.RLock()
	target := cfg.Target
	mu.RUnlock()
	u, _ := validTarget(target)
	status := 200
	p := &httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) {
		pr.SetURL(u)
		pr.Out.Host = u.Host
		pr.Out.Header.Del("Authorization")
		pr.Out.Header.Del("Cookie")
		pr.Out.Header.Del("Forwarded")
		pr.Out.Header.Del("X-Forwarded-For")
		pr.Out.Header.Del("X-Forwarded-Host")
		pr.Out.Header.Del("X-Forwarded-Proto")
	}, Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext, ForceAttemptHTTP2: true, MaxIdleConns: 50, IdleConnTimeout: 60 * time.Second, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 30 * time.Second}, ModifyResponse: func(res *http.Response) error { status = res.StatusCode; res.Header.Del("Set-Cookie"); return nil }, ErrorHandler: func(w http.ResponseWriter, r *http.Request, e error) {
		status = 502
		http.Error(w, "Hedef servise ulaşılamadı", 502)
	}}
	// Do not capture body, query strings, credentials or headers.
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	p.ServeHTTP(w, r)
	path := r.URL.EscapedPath()
	if len(path) > 160 {
		path = path[:160]
	}
	// Restrict metadata to known entry path; do not persist user-controlled path segments.
	if strings.HasPrefix(path, "/login/") {
		path = "/login/*"
	}
	mu.Lock()
	count++
	events = append([]Event{{Time: time.Now().UTC().Format(time.RFC3339), Method: r.Method, Path: path, Status: status, Duration: time.Since(start).Milliseconds()}}, events...)
	if len(events) > 100 {
		events = events[:100]
	}
	mu.Unlock()
}

var _ = bytes.MinRead
