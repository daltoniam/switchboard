package imessage

import (
	"bytes"
	"encoding/binary"
	"strings"
	"unicode/utf8"
)

var nsStringMarkers = [][]byte{[]byte("NSString"), []byte("NSMutableString")}

// maxPlusSearch bounds how far past the class name the '+' (C string type
// tag) may appear before the length prefix.
const maxPlusSearch = 8

// decodeAttributedBody extracts the plain text from the NSAttributedString
// typedstream archive that Messages stores in message.attributedBody when the
// text column is empty (most messages on modern macOS).
func decodeAttributedBody(blob []byte) (string, bool) {
	for _, marker := range nsStringMarkers {
		idx := bytes.Index(blob, marker)
		if idx < 0 {
			continue
		}
		if s, ok := readTypedString(blob[idx+len(marker):]); ok {
			return s, true
		}
	}
	return "", false
}

func readTypedString(rest []byte) (string, bool) {
	limit := min(len(rest), maxPlusSearch)
	plus := bytes.IndexByte(rest[:limit], '+')
	if plus < 0 {
		return "", false
	}
	rest = rest[plus+1:]
	if len(rest) == 0 {
		return "", false
	}
	var n int
	switch rest[0] {
	case 0x81:
		if len(rest) < 3 {
			return "", false
		}
		n = int(binary.LittleEndian.Uint16(rest[1:3]))
		rest = rest[3:]
	case 0x82:
		if len(rest) < 5 {
			return "", false
		}
		n = int(binary.LittleEndian.Uint32(rest[1:5]))
		rest = rest[5:]
	default:
		n = int(rest[0])
		rest = rest[1:]
	}
	if n < 0 || n > len(rest) {
		return "", false
	}
	raw := rest[:n]
	if !utf8.Valid(raw) {
		return "", false
	}
	return strings.ReplaceAll(string(raw), "\ufffc", ""), true
}
