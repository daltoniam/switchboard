package awmgrpc

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	awmv1 "github.com/daltoniam/switchboard/gen/awm/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestMultiplexHTTPAndGRPC_PreservesHTTP(t *testing.T) {
	grpcServer := grpc.NewServer()
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "http") })
	h := MultiplexHTTPAndGRPC(grpcServer, httpHandler)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "http", rr.Body.String())
}

func TestMultiplexHTTPAndGRPC_RoutesNativeGRPC(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	Register(grpcServer, nil, nil, nil, nil, Options{})
	server := &http.Server{Handler: MultiplexHTTPAndGRPC(grpcServer, http.NotFoundHandler())}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = awmv1.NewWorkProfileServiceClient(conn).ListWorkProfiles(context.Background(), &awmv1.ListWorkProfilesRequest{})
	assert.Equal(t, codes.Unavailable, status.Code(err))
}
