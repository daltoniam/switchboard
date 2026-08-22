package project

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ContextEntry describes a single file in an assembled context bundle.
type ContextEntry struct {
	Path     string `json:"path"`
	Source   string `json:"source"`
	MIMEType string `json:"mimeType"`
	Size     int    `json:"sizeBytes"`
}

// AssembleManifest returns no context entries for id+description projects.
func AssembleManifest(def *Definition, configDir string) []ContextEntry {
	return []ContextEntry{}
}

// AssembleManifestAtRoot returns no context entries.
func AssembleManifestAtRoot(def *Definition, configDir, root string) []ContextEntry {
	return []ContextEntry{}
}

// AssembleManifestWithRole returns no context entries unless role overrides list files.
func AssembleManifestWithRole(def *Definition, configDir string, role string) []ContextEntry {
	return []ContextEntry{}
}

// ReadContextFile reports not found for the simplified schema.
func ReadContextFile(def *Definition, configDir, path string) (string, error) {
	return "", fmt.Errorf("context file not found: %s", path)
}

// GuessMIME returns a MIME type based on file extension.
func GuessMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "text/yaml"
	default:
		return "text/plain"
	}
}
