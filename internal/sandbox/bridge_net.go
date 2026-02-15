package sandbox

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dop251/goja"
)

const maxResponseBody = 100 * 1024 * 1024 // 100MB

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:  10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	},
}

func (s *Sandbox) registerNet(vm *goja.Runtime) {
	net := vm.NewObject()

	net.Set("fetch", func(call goja.FunctionCall) goja.Value {
		// Check network access approval
		if s.cfg.ApproveNet != nil {
			allowed, err := s.cfg.ApproveNet()
			if err != nil {
				s.checkInterrupted(err)
				throwError(vm, fmt.Sprintf("net.fetch: %s", err.Error()))
			}
			if !allowed {
				throwError(vm, "net.fetch: network access denied")
			}
		} else {
			throwError(vm, "net.fetch: network access denied (no approval handler)")
		}

		url := call.Argument(0).String()

		// Parse options (method, headers, body)
		method := "GET"
		var body io.Reader
		var headers map[string]string

		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Argument(1)) && !goja.IsNull(call.Argument(1)) {
			opts := call.Argument(1).ToObject(vm)

			if m := opts.Get("method"); m != nil && !goja.IsUndefined(m) {
				method = strings.ToUpper(m.String())
			}
			if b := opts.Get("body"); b != nil && !goja.IsUndefined(b) {
				body = strings.NewReader(b.String())
			}
			if h := opts.Get("headers"); h != nil && !goja.IsUndefined(h) {
				headers = make(map[string]string)
				hObj := h.ToObject(vm)
				for _, key := range hObj.Keys() {
					headers[key] = hObj.Get(key).String()
				}
			}
		}

		req, err := http.NewRequestWithContext(s.ctx, method, url, body)
		if err != nil {
			throwError(vm, fmt.Sprintf("net.fetch: invalid request: %s", err.Error()))
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			throwError(vm, fmt.Sprintf("net.fetch: request to %s failed: %s", url, err.Error()))
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
		if err != nil {
			throwError(vm, fmt.Sprintf("net.fetch: error reading response from %s", url))
		}
		if int64(len(respBody)) > maxResponseBody {
			throwError(vm, fmt.Sprintf("net.fetch: response body from %s exceeds %dMB limit", url, maxResponseBody/1024/1024))
		}

		// Convert response headers to a plain object
		respHeaders := make(map[string]string)
		for k := range resp.Header {
			respHeaders[strings.ToLower(k)] = resp.Header.Get(k)
		}

		return vm.ToValue(map[string]any{
			"status":  resp.StatusCode,
			"headers": respHeaders,
			"body":    string(respBody),
		})
	})

	vm.Set("net", net)
}
