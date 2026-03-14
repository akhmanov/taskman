package model

type ArtifactState struct {
	Version int               `yaml:"version"`
	Data    map[string]string `yaml:"data,omitempty"`
}
