package main

import (
	"embed"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed assets/template.html
var templateFS embed.FS

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fs.Int("port", 8787, "HTTP port")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Find available port
	for {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
		if err == nil {
			ln.Close()
			break
		}
		*port++
	}

	templateBytes, err := templateFS.ReadFile("assets/template.html")
	if err != nil {
		return fmt.Errorf("embedded template not found: %w", err)
	}

	html := string(templateBytes)
	html = strings.ReplaceAll(html, "__AUTORESEARCH_TITLE__", "Autoresearch Dashboard")
	html = strings.ReplaceAll(html, "__AUTORESEARCH_LOGO__", "")

	// SSE client management
	var mu sync.Mutex
	clients := make(map[chan struct{}]struct{})
	addClient := func() chan struct{} {
		ch := make(chan struct{}, 1)
		mu.Lock()
		clients[ch] = struct{}{}
		mu.Unlock()
		return ch
	}
	removeClient := func(ch chan struct{}) {
		mu.Lock()
		delete(clients, ch)
		mu.Unlock()
	}
	broadcast := func() {
		mu.Lock()
		for ch := range clients {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
		mu.Unlock()
	}

	// Watch jsonl file for changes
	go func() {
		var lastMod time.Time
		for {
			time.Sleep(time.Second)
			info, err := os.Stat(jsonlFile)
			if err != nil {
				continue
			}
			if info.ModTime().After(lastMod) {
				lastMod = info.ModTime()
				broadcast()
			}
		}
	}()

	mux := http.NewServeMux()

	// SSE endpoint
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		fmt.Fprint(w, "event: connected\ndata: ok\n\n")
		flusher.Flush()

		ch := addClient()
		defer removeClient(ch)

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				fmt.Fprint(w, "event: jsonl-updated\ndata: updated\n\n")
				flusher.Flush()
			}
		}
	})

	// JSONL data endpoint
	mux.HandleFunc("/autoresearch.jsonl", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(jsonlFile)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(data)
	})

	// Dashboard HTML
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Cache-Control", "no-cache")
		fmt.Fprint(w, html)
	})

	fmt.Printf("🔬 Autoresearch dashboard starting on http://localhost:%d\n", *port)
	fmt.Printf("   Serving: %s/%s\n", mustGetwd(), jsonlFile)
	fmt.Println("   Press Ctrl+C to stop")
	fmt.Println()

	return http.ListenAndServe(fmt.Sprintf(":%d", *port), mux)
}

func mustGetwd() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	return d
}
