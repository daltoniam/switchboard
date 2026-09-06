package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gowebpki/jcs"
)

var revisionRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// ParseRevision validates the frozen sha256:<hex> spelling.
func ParseRevision(r Revision) (Revision, error) {
	s := string(r)
	if !revisionRE.MatchString(s) {
		return "", fmt.Errorf("invalid revision %q", s)
	}
	return r, nil
}

// RawSourceRevision hashes the exact malformed user-file bytes.
func RawSourceRevision(raw []byte) Revision {
	sum := sha256.Sum256(raw)
	return Revision("sha256:" + hex.EncodeToString(sum[:]))
}

// CanonicalJSON encodes v with RFC 8785 JCS. Nested json.RawMessage is
// decoded and recanonicalized so equivalent numeric spellings and key
// orders hash identically.
func CanonicalJSON(v any) ([]byte, error) {
	normalized, err := normalizeForJCS(v)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal for JCS: %w", err)
	}
	canon, err := jcs.Transform(raw)
	if err != nil {
		return nil, fmt.Errorf("jcs transform: %w", err)
	}
	return canon, nil
}

func normalizeForJCS(v any) (any, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case json.RawMessage:
		if len(t) == 0 || string(t) == "null" {
			return nil, nil
		}
		var decoded any
		if err := json.Unmarshal(t, &decoded); err != nil {
			return nil, err
		}
		return normalizeForJCS(decoded)
	case json.Number:
		s := string(t)
		if strings.EqualFold(s, "NaN") || strings.EqualFold(s, "Infinity") || strings.EqualFold(s, "+Infinity") || strings.EqualFold(s, "-Infinity") {
			return nil, fmt.Errorf("jcs cannot represent %q", s)
		}
		var decoded any
		if err := json.Unmarshal([]byte(s), &decoded); err != nil {
			return nil, fmt.Errorf("jcs cannot represent number %q: %w", s, err)
		}
		return decoded, nil
	case map[string]json.RawMessage:
		out := make(map[string]any, len(t))
		for k, raw := range t {
			nv, err := normalizeForJCS(raw)
			if err != nil {
				return nil, err
			}
			out[k] = nv
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			nv, err := normalizeForJCS(val)
			if err != nil {
				return nil, err
			}
			out[k] = nv
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			nv, err := normalizeForJCS(val)
			if err != nil {
				return nil, err
			}
			out[i] = nv
		}
		return out, nil
	case float32:
		if isNonFiniteFloat(float64(t)) {
			return nil, fmt.Errorf("jcs cannot represent non-finite float")
		}
		return t, nil
	case float64:
		if isNonFiniteFloat(t) {
			return nil, fmt.Errorf("jcs cannot represent non-finite float")
		}
		return t, nil
	default:
		return v, nil
	}
}

func isNonFiniteFloat(f float64) bool {
	return f != f || f > 1.7976931348623157e+308 || f < -1.7976931348623157e+308
}

// HashBytes returns sha256:<hex> of already-canonical JSON bytes.
func HashBytes(canon []byte) Revision {
	sum := sha256.Sum256(canon)
	return Revision("sha256:" + hex.EncodeToString(sum[:]))
}

// HashDefinition hashes the definition as both effective revision and
// user-layer sourceRevision. Callers that merge overlays must hash the
// user layer and effective layer separately.
func HashDefinition(def *Definition) (revision, sourceRevision Revision, err error) {
	if def == nil {
		return "", "", fmt.Errorf("definition is nil")
	}
	canon, err := CanonicalJSON(def)
	if err != nil {
		return "", "", err
	}
	h := HashBytes(canon)
	return h, h, nil
}

// HashUserAndEffective hashes the user-layer definition and the effective
// merged definition independently.
func HashUserAndEffective(user, effective *Definition) (revision, sourceRevision Revision, err error) {
	userCanon, err := CanonicalJSON(user)
	if err != nil {
		return "", "", err
	}
	effCanon, err := CanonicalJSON(effective)
	if err != nil {
		return "", "", err
	}
	return HashBytes(effCanon), HashBytes(userCanon), nil
}
