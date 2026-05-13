package suno

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("suno", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type suno struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

var (
	_ mcp.FieldCompactionIntegration = (*suno)(nil)
	_ mcp.PlainTextCredentials       = (*suno)(nil)
	_ mcp.PlaceholderHints           = (*suno)(nil)
	_ mcp.OptionalCredentials        = (*suno)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*suno)(nil)
)

func (s *suno) PlainTextKeys() []string { return []string{"base_url"} }
func (s *suno) Placeholders() map[string]string {
	return map[string]string{"base_url": "https://api.sunoapi.org (default)"}
}
func (s *suno) OptionalKeys() []string { return []string{"base_url"} }

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &suno{
		client:  &http.Client{Timeout: 120 * time.Second},
		baseURL: "https://api.sunoapi.org",
	}
}

func (s *suno) Name() string { return "suno" }

func (s *suno) Configure(_ context.Context, creds mcp.Credentials) error {
	s.apiKey = creds["api_key"]
	if s.apiKey == "" {
		return fmt.Errorf("suno: api_key is required")
	}
	if v := creds["base_url"]; v != "" {
		s.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (s *suno) Healthy(ctx context.Context) bool {
	_, err := s.get(ctx, "/api/v1/generate/credit")
	return err == nil
}

func (s *suno) Tools() []mcp.ToolDefinition {
	return tools
}

func (s *suno) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, s, args)
}

func (s *suno) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (s *suno) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

// --- HTTP helpers ---

func (s *suno) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("suno API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (s *suno) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return s.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (s *suno) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return s.doRequest(ctx, "POST", path, body)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, s *suno, args map[string]any) (*mcp.ToolResult, error)

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// Music generation
	mcp.ToolName("suno_generate_music"): generateMusic,
	mcp.ToolName("suno_get_generation"): getGeneration,
	mcp.ToolName("suno_extend_music"):   extendMusic,
	mcp.ToolName("suno_get_credits"):    getCredits,

	// Lyrics
	mcp.ToolName("suno_generate_lyrics"):    generateLyrics,
	mcp.ToolName("suno_get_lyrics"):         getLyrics,
	mcp.ToolName("suno_get_aligned_lyrics"): getAlignedLyrics,

	// Audio processing
	mcp.ToolName("suno_separate_stems"):      separateStems,
	mcp.ToolName("suno_get_stem_separation"): getStemSeparation,
	mcp.ToolName("suno_convert_wav"):         convertWav,
	mcp.ToolName("suno_get_wav_conversion"):  getWavConversion,

	// Advanced generation
	mcp.ToolName("suno_cover_audio"):      coverAudio,
	mcp.ToolName("suno_upload_extend"):    uploadExtend,
	mcp.ToolName("suno_add_vocals"):       addVocals,
	mcp.ToolName("suno_add_instrumental"): addInstrumental,
	mcp.ToolName("suno_generate_mashup"):  generateMashup,

	// Persona
	mcp.ToolName("suno_generate_persona"): generatePersona,

	// Video
	mcp.ToolName("suno_generate_video"): generateVideo,
	mcp.ToolName("suno_get_video"):      getVideo,

	// MIDI
	mcp.ToolName("suno_generate_midi"): generateMidi,
}
