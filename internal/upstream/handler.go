package upstream

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"newapiguard/internal/settings"
	"newapiguard/internal/webutil"
)

const anthropicUpstream = "https://api.anthropic.com"

type Handler struct {
	settings *settings.Store
	client   *http.Client
}

func NewHandler(settingsStore *settings.Store) *Handler {
	return &Handler{
		settings: settingsStore,
		client:   &http.Client{},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		webutil.WriteError(w, http.StatusBadRequest, "读取请求体失败")
		return
	}

	if h.settings.GetBool("prompt_cache_enabled", true) {
		bodyBytes = injectCacheControl(bodyBytes)
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
