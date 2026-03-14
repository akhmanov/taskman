package model

type ArtifactState struct {
	Version int               `json:"version" yaml:"version"`
	Data    map[string]string `json:"data,omitempty" yaml:"data,omitempty"`
}
