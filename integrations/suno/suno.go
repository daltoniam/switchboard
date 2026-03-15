package suno

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type suno struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

var _ mcp.FieldCompactionIntegration = (*suno)(nil)

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

func (s *suno) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, s, args)
}

func (s *suno) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
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

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

// --- Argument helpers ---

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

func argFloat(args map[string]any, key string) float64 {
	switch v := args[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Music generation
	"suno_generate_music": generateMusic,
	"suno_get_generation": getGeneration,
	"suno_extend_music":   extendMusic,
	"suno_get_credits":    getCredits,

	// Lyrics
	"suno_generate_lyrics":    generateLyrics,
	"suno_get_lyrics":         getLyrics,
	"suno_get_aligned_lyrics": getAlignedLyrics,

	// Audio processing
	"suno_separate_stems":      separateStems,
	"suno_get_stem_separation": getStemSeparation,
	"suno_convert_wav":         convertWav,
	"suno_get_wav_conversion":  getWavConversion,

	// Advanced generation
	"suno_cover_audio":      coverAudio,
	"suno_upload_extend":    uploadExtend,
	"suno_add_vocals":       addVocals,
	"suno_add_instrumental": addInstrumental,
	"suno_generate_mashup":  generateMashup,

	// Persona
	"suno_generate_persona": generatePersona,

	// Video
	"suno_generate_video": generateVideo,
	"suno_get_video":      getVideo,

	// MIDI
	"suno_generate_midi": generateMidi,
}
