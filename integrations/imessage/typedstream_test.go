package imessage

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func buildAttributedBody(text string) []byte {
	out := []byte("\x04\x0bstreamtyped\x81\xe8\x03\x84\x01@\x84\x84\x84\x12NSAttributedString\x00\x84\x84\x08NSObject\x00\x85\x92\x84\x84\x84\x08NSString")
	out = append(out, 0x01, 0x94, 0x84, 0x01, '+')
	n := len(text)
	switch {
	case n < 0x80:
		out = append(out, byte(n))
	case n <= 0xFFFF:
		out = append(out, 0x81)
		out = binary.LittleEndian.AppendUint16(out, uint16(n))
	default:
		out = append(out, 0x82)
		out = binary.LittleEndian.AppendUint32(out, uint32(n))
	}
	out = append(out, text...)
	out = append(out, 0x86, 0x84, 0x02, 'i', 'I', 0x01)
	return out
}

func TestDecodeAttributedBody(t *testing.T) {
	long := strings.Repeat("a", 300)
	huge := strings.Repeat("b", 70000)
	tests := []struct {
		name string
		blob []byte
		want string
		ok   bool
	}{
		{name: "short", blob: buildAttributedBody("hello"), want: "hello", ok: true},
		{name: "utf8", blob: buildAttributedBody("café 👍"), want: "café 👍", ok: true},
		{name: "two byte length", blob: buildAttributedBody(long), want: long, ok: true},
		{name: "four byte length", blob: buildAttributedBody(huge), want: huge, ok: true},
		{name: "object replacement stripped", blob: buildAttributedBody("\ufffcphoto"), want: "photo", ok: true},
		{name: "empty", blob: nil, ok: false},
		{name: "no NSString", blob: []byte("streamtyped garbage"), ok: false},
		{name: "truncated length", blob: []byte("NSString\x01\x94\x84\x01+\x81\x05"), ok: false},
		{name: "length past end", blob: []byte("NSString\x01\x94\x84\x01+\x10abc"), ok: false},
		{name: "missing plus marker", blob: []byte("NSString\x01\x94\x84\x01\x01\x01\x01\x01\x01\x01"), ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := decodeAttributedBody(tt.blob)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
