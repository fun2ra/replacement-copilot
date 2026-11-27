// Package config persists the IDs created by cmd/setup. Agents are versioned,
// persistent objects: create once, store the ID, reuse it on every session.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

const File = "agent.json"

type AgentConfig struct {
	AgentID       string `json:"agent_id"`
	AgentVersion  int64  `json:"agent_version"`
	EnvironmentID string `json:"environment_id"`
}

func Load() (AgentConfig, error) {
	var c AgentConfig
	raw, err := os.ReadFile(File)
	if err != nil {
		return c, fmt.Errorf("read %s (run `go run ./cmd/setup` first): %w", File, err)
	}
	return c, json.Unmarshal(raw, &c)
}

func Save(c AgentConfig) error {
	raw, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(File, raw, 0o644)
}
