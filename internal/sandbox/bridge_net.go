package sandbox

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dop251/goja"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	},
}

// isPrivateIP checks if an IP address is in a private/internal range (SSRF protection)
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	// Loopback
	if ip.IsLoopback() {
		return true
	}
	// Link-local
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	// Private ranges: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 10 {
			return true
		}
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		// 127.0.0.0/8 (loopback range)
		if ip4[0] == 127 {
			return true
		}
	}
	return false
}

func (s *Sandbox) registerNet(vm *goja.Runtime) {
	netObj := vm.NewObject()

	netObj.Set("fetch", func(call goja.FunctionCall) goja.Value {
		urlStr := call.Argument(0).String()

		// Extract host from URL for approval
		parsedURL, err := url.Parse(urlStr)
		if err != nil {
			throwError(vm, fmt.Sprintf("net.fetch: invalid URL: %s", err.Error()))
		}
		host := parsedURL.Hostname()

		// SSRF protection: block requests to private/internal IPs
		if ip := net.ParseIP(host); ip != nil {
			if isPrivateIP(ip) {
				throwError(vm, fmt.Sprintf("net.fetch: access to private IP %s denied", host))
			}
		} else {
			// Resolve hostname and check if it points to private IP
			ips, err := net.LookupIP(host)
			if err == nil {
				for _, ip := range ips {
					if isPrivateIP(ip) {
						throwError(vm, fmt.Sprintf("net.fetch: %s resolves to private IP, access denied", host))
					}
				}
			}
		}

		// Check network access approval
		if s.cfg.ApproveNet != nil {
			allowed, err := s.cfg.ApproveNet(host)
			if err != nil {
				s.checkInterrupted(err)
				throwError(vm, fmt.Sprintf("net.fetch: %s", err.Error()))
			}
			if !allowed {
				throwError(vm, fmt.Sprintf("net.fetch: access to %s denied", host))
			}
		} else {
			throwError(vm, "net.fetch: network access denied (no approval handler)")
		}

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

		req, err := http.NewRequestWithContext(s.ctx, method, urlStr, body)
		if err != nil {
			throwError(vm, fmt.Sprintf("net.fetch: invalid request: %s", err.Error()))
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			throwError(vm, fmt.Sprintf("net.fetch: request to %s failed: %s", urlStr, err.Error()))
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, MaxNetRespSize+1))
		if err != nil {
			throwError(vm, fmt.Sprintf("net.fetch: error reading response from %s", urlStr))
		}
		if int64(len(respBody)) > MaxNetRespSize {
			throwError(vm, fmt.Sprintf("net.fetch: response body from %s exceeds %dMB limit", urlStr, MaxNetRespSize>>20))
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

	vm.Set("net", netObj)
}
