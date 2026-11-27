// Command setup is the control-plane step: it creates (or updates) the
// Managed Agents environment and agent once and stores their IDs in
// agent.json. The server only ever creates sessions against those IDs.
//
//	go run ./cmd/setup
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"replacement-copilot/internal/config"
	"replacement-copilot/internal/tools"
)

const model = "claude-opus-5-5"

func main() {
	ctx := context.Background()
	client := anthropic.NewClient() // reads ANTHROPIC_API_KEY

	cfg, err := config.Load()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatal(err)
	}

	if cfg.EnvironmentID == "" {
		// The agent has no sandbox tools, so the container needs no network.
		env, err := client.Beta.Environments.New(ctx, anthropic.BetaEnvironmentNewParams{
			Name: "replacement-copilot-env",
			Config: anthropic.BetaEnvironmentNewParamsConfigUnion{
				OfCloud: &anthropic.BetaCloudConfigParams{
					Networking: anthropic.BetaCloudConfigParamsNetworkingUnion{
						OfLimited: &anthropic.BetaLimitedNetworkParams{
							AllowPackageManagers: anthropic.Bool(false),
							AllowMCPServers:      anthropic.Bool(false),
						},
					},
				},
			},
		})
		if err != nil {
			log.Fatalf("create environment: %v", err)
		}
		cfg.EnvironmentID = env.ID
		fmt.Println("created environment", env.ID)
	}

	if cfg.AgentID == "" {
		agent, err := client.Beta.Agents.New(ctx, anthropic.BetaAgentNewParams{
			Name:   "Replacement Copilot",
			Model:  anthropic.BetaManagedAgentsModelConfigParams{ID: model},
			System: anthropic.String(tools.SystemPrompt),
			Tools:  tools.ForCreate(),
		})
		if err != nil {
			log.Fatalf("create agent: %v", err)
		}
		cfg.AgentID, cfg.AgentVersion = agent.ID, agent.Version
		fmt.Printf("created agent %s (version %d)\n", agent.ID, agent.Version)
	} else {
		// Re-running setup after editing the prompt or tools creates a new
		// agent version; running sessions keep the version they started with.
		agent, err := client.Beta.Agents.Update(ctx, cfg.AgentID, anthropic.BetaAgentUpdateParams{
			Version: anthropic.Int(cfg.AgentVersion),
			System:  anthropic.String(tools.SystemPrompt),
			Tools:   tools.ForUpdate(),
		})
		if err != nil {
			log.Fatalf("update agent: %v", err)
		}
		cfg.AgentVersion = agent.Version
		fmt.Printf("agent %s is now at version %d\n", agent.ID, agent.Version)
	}

	if err := config.Save(cfg); err != nil {
		log.Fatal(err)
	}
	fmt.Println("saved", config.File)
}
