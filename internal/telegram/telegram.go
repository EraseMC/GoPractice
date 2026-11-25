package telegram

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/EraseMC/GoPractice/internal/config"
)

type Client struct {
	log     *slog.Logger
	token   string
	chatID  string
	http    *http.Client
	enabled bool
}

func New(log *slog.Logger, env config.Env) *Client {
	c := &Client{
		log:    log,
		token:  env.Get("OLDTRALIA_TELEGRAM_TOKEN", ""),
		chatID: env.Get("OLDTRALIA_TELEGRAM_CHAT_ID", ""),
	}
	c.enabled = c.token != "" && c.chatID != ""
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy := env.Get("OLDTRALIA_TELEGRAM_PROXY", ""); proxy != "" {
		if !strings.Contains(proxy, "://") {
			proxy = "http://" + proxy
		}
		if u, err := url.Parse(proxy); err == nil {
			transport.Proxy = http.ProxyURL(u)
		} else {
			log.Warn("invalid telegram proxy", "err", err)
		}
	}
	c.http = &http.Client{Timeout: 10 * time.Second, Transport: transport}
	return c
}

func (c *Client) Enabled() bool {
	return c != nil && c.enabled
}

func (c *Client) Send(message string) {
	if !c.Enabled() {
		return
	}
	link := "https://api.telegram.org/bot" + c.token + "/sendMessage?" + url.Values{
		"chat_id": {c.chatID},
		"text":    {message},
	}.Encode()
	go func() {
		resp, err := c.http.Get(link)
		if err != nil {
			if ue, ok := err.(*url.Error); ok {
				err = ue.Err
			}
			c.log.Debug("telegram request failed", "err", err)
			return
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
}

func Date() string {
	return time.Now().Format("02.01.06")
}
