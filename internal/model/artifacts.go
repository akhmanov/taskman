package model

type ArtifactState struct {
	Version int            `json:"version" yaml:"version"`
	Data    map[string]any `json:"data,omitempty" yaml:"data,omitempty"`
}
