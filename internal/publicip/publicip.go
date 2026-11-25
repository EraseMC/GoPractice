package publicip

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var sources = []string{"http://api.ipify.org/", "http://ifconfig.me/ip"}

type Resolver struct {
	mu sync.Mutex
	ip string
}

func (r *Resolver) Get() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ip
}

func (r *Resolver) Resolve(ctx context.Context) string {
	if ip := r.Get(); ip != "" {
		return ip
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for _, url := range sources {
		ip := fetch(ctx, client, url)
		if ip == "" {
			continue
		}
		r.mu.Lock()
		r.ip = ip
		r.mu.Unlock()
		return ip
	}
	return ""
}

func fetch(ctx context.Context, client *http.Client, url string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return ""
	}
	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}
