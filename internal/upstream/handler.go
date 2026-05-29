package upstream

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"newapiguard/internal/promptcache"
	"newapiguard/internal/settings"
	"newapiguard/internal/webutil"
)

const (
	anthropicUpstream = "https://api.anthropic.com"
	maxRequestBody    = 20 << 20 // 20 MB
)

type Handler struct {
	settings *settings.Store
	client   *http.Client
}

func NewHandler(settingsStore *settings.Store) *Handler {
	return &Handler{
		settings: settingsStore,
		client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		webutil.WriteError(w, http.StatusMethodNotAllowed, "仅支持 POST 请求")
		return
	}

	bodyBytes, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
	r.Body.Close()
	if err != nil {
		webutil.WriteError(w, http.StatusBadRequest, "读取请求体失败或请求体过大")
		return
	}

	var report promptcache.Report
	if h.settings.GetBool("prompt_cache_enabled", true) {
		bodyBytes, report, _ = promptcache.Inject(bodyBytes, promptcache.Options{})
	}

	if h.settings.GetBool("prompt_cache_debug", false) {
		promptcache.LogReport("/upstream/anthropic"+r.URL.Path, report)
	}

	upstreamURL := anthropicUpstream + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	upReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		webutil.WriteError(w, http.StatusInternalServerError, "构建上游请求失败")
		return
	}

	webutil.CloneHeader(upReq.Header, r.Header)
	upReq.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))
	upReq.ContentLength = int64(len(bodyBytes))

	resp, err := h.client.Do(upReq)
	if err != nil {
		log.Printf("[upstream] 请求 Anthropic 失败: %v", err)
		webutil.WriteError(w, http.StatusBadGateway, "上游服务不可用")
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		flusher, ok := w.(http.Flusher)
		buf := make([]byte, 4096)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				if ok {
					flusher.Flush()
				}
			}
			if readErr != nil {
				break
			}
		}
	} else {
		io.Copy(w, resp.Body)
	}
}
