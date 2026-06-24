package backend

import (
	"net"
	"net/http"
	"time"
)

// Connection guards against an unreachable or unresponsive backend. They bound
// connection setup and time-to-first-byte (response headers) without capping
// total stream duration, so a slow-but-valid streamed response is never
// truncated. User-initiated aborts are handled separately via context.
const (
	dialTimeout           = 10 * time.Second
	responseHeaderTimeout = 60 * time.Second
)

// httpClient is shared by all HTTP-based backends.
var httpClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: dialTimeout}).DialContext,
		TLSHandshakeTimeout:   dialTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
	},
}
