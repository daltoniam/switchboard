package agents

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// grpcClient makes gRPC calls over HTTP/2 cleartext (h2c) to the ARP daemon.
// It uses google.protobuf.Struct for all request/response encoding to avoid
// importing the full ARP proto definitions. The daemon's gRPC services accept
// JSON-encoded Struct messages via the standard protobuf JSON mapping.
type grpcClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func newGRPCClient(baseURL, token string) *grpcClient {
	return &grpcClient{
		baseURL: baseURL,
		token:   token,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http2.Transport{
				AllowHTTP: true,
				DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, network, addr)
				},
			},
		},
	}
}

// call invokes a gRPC method using protobuf Struct encoding over h2c.
// The request args are encoded as a google.protobuf.Struct, framed in gRPC
// wire format, and sent as application/grpc. The response is decoded from
// the gRPC frame back into a JSON-compatible map.
func (g *grpcClient) call(ctx context.Context, service, method string, args map[string]any) (json.RawMessage, error) {
	// Encode request as protobuf Struct.
	reqStruct, err := structpb.NewStruct(args)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	reqBytes, err := proto.Marshal(reqStruct)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Frame in gRPC wire format: 1 byte compressed flag + 4 byte length + payload.
	frame := make([]byte, 5+len(reqBytes))
	frame[0] = 0 // not compressed
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(reqBytes)))
	copy(frame[5:], reqBytes)

	url := fmt.Sprintf("%s/%s/%s", g.baseURL, service, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, io.NopCloser(bytesReader(frame)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("TE", "trailers")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("grpc call %s/%s: %w", service, method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		gs := resp.Trailer.Get("grpc-status")
		gm := resp.Trailer.Get("grpc-message")
		if gs == "" {
			gs = resp.Header.Get("grpc-status")
		}
		if gm == "" {
			gm = resp.Header.Get("grpc-message")
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if gm == "" {
			gm = string(body)
		}
		return nil, fmt.Errorf("gRPC error (http=%d, status=%s): %s", resp.StatusCode, gs, gm)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check trailers for gRPC status.
	if gs := resp.Trailer.Get("grpc-status"); gs != "" && gs != "0" {
		gm := resp.Trailer.Get("grpc-message")
		if gm == "" {
			gm = "unknown error"
		}
		return nil, fmt.Errorf("gRPC error (status=%s): %s", gs, gm)
	}

	// Empty response is valid (e.g. for delete operations).
	if len(respBody) < 5 {
		// Check header-based grpc-status for responses with no body.
		if gs := resp.Header.Get("grpc-status"); gs != "" && gs != "0" {
			gm := resp.Header.Get("grpc-message")
			return nil, fmt.Errorf("gRPC error (status=%s): %s", gs, gm)
		}
		return json.RawMessage("{}"), nil
	}

	// Parse gRPC frame.
	if respBody[0]&0x80 != 0 {
		// Trailers-only frame.
		return json.RawMessage("{}"), nil
	}

	msgLen := binary.BigEndian.Uint32(respBody[1:5])
	if len(respBody) < int(5+msgLen) {
		return nil, fmt.Errorf("gRPC response truncated: have %d, need %d", len(respBody)-5, msgLen)
	}

	// Decode as Struct and convert to JSON.
	var respStruct structpb.Struct
	if err := proto.Unmarshal(respBody[5:5+msgLen], &respStruct); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	jsonBytes, err := protojson.Marshal(&respStruct)
	if err != nil {
		return nil, fmt.Errorf("marshal response to JSON: %w", err)
	}

	return json.RawMessage(jsonBytes), nil
}

// bytesReader wraps a byte slice as an io.Reader.
func bytesReader(b []byte) io.Reader {
	return &bytesReaderImpl{data: b}
}

type bytesReaderImpl struct {
	data []byte
	pos  int
}

func (r *bytesReaderImpl) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
