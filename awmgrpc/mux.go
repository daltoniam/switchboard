package awmgrpc

import (
	"net/http"
	"strings"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// MultiplexHTTPAndGRPC serves native gRPC and the existing HTTP/MCP routes on
// one cleartext port. gRPC requests are identified by HTTP/2 plus the standard
// application/grpc content type; every other request is delegated unchanged.
func MultiplexHTTPAndGRPC(grpcHandler, httpHandler http.Handler) http.Handler {
	if grpcHandler == nil {
		return httpHandler
	}
	if httpHandler == nil {
		httpHandler = http.NotFoundHandler()
	}
	dispatch := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/grpc") {
			grpcHandler.ServeHTTP(w, r)
			return
		}
		httpHandler.ServeHTTP(w, r)
	})
	return h2c.NewHandler(dispatch, &http2.Server{})
}
