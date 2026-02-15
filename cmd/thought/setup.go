package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/thinkingscript/cli/internal/config"
	"github.com/thinkingscript/cli/internal/ui"
)

var (
	setupAPIKeyFlag string
	setupModelFlag  string
)

var setupCmd = &cobra.Command{
	Use:          "setup",
	Short:        "Configure thinkingscript with your API key and model",
	RunE:         runSetup,
	SilenceUsage: true,
}

func init() {
	setupCmd.Flags().StringVar(&setupAPIKeyFlag, "api-key", "", "Anthropic API key")
	setupCmd.Flags().StringVar(&setupModelFlag, "model", "", "Model to use (e.g. claude-sonnet-4-5-20250929)")
}

func runSetup(cmd *cobra.Command, args []string) error {
	if err := config.EnsureHomeDir(); err != nil {
		return fmt.Errorf("initializing home directory: %w", err)
	}

	existing := config.LoadAgent("anthropic")

	apiKey := setupAPIKeyFlag
	model := setupModelFlag

	if apiKey == "" || model == "" {
		// Interactive mode
		var err error
		if apiKey == "" {
			apiKey, err = promptAPIKey(existing.APIKey)
			if err != nil {
				return err
			}
		}
		if model == "" {
			model, err = promptModel(existing.Model)
			if err != nil {
				return err
			}
		}
	}

	// Validate the API key
	valid := validateAPIKey(cmd.Context(), apiKey)
	if !valid {
		fmt.Fprintln(os.Stderr, "\n  API key validation failed.")
		if setupAPIKeyFlag != "" {
			// Non-interactive: just warn and save anyway
			fmt.Fprintln(os.Stderr, "  Saving anyway (network may be unavailable).")
		} else {
			saveAnyway, err := promptSaveAnyway()
			if err != nil {
				return err
			}
			if !saveAnyway {
				fmt.Fprintln(os.Stderr, "  Setup cancelled.")
				return nil
			}
		}
	}

	agent := &config.AgentConfig{
		Version:  1,
		Provider: "anthropic",
		APIKey:   apiKey,
		Model:    model,
	}
	if err := config.SaveAgent("anthropic", agent); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	configPath := filepath.Join(config.HomeDir(), "agents", "anthropic.yaml")
	fmt.Fprintf(os.Stderr, "\n  Config saved to %s\n", configPath)
	fmt.Fprintf(os.Stderr, "  Try it out: think examples/weather.md \"San Francisco\"\n")
	return nil
}

func promptAPIKey(existing string) (string, error) {
	var apiKey string
	placeholder := "sk-ant-..."
	description := "Enter your Anthropic API key"
	if existing != "" {
		if len(existing) > 11 {
			placeholder = existing[:7] + strings.Repeat("*", len(existing)-11) + existing[len(existing)-4:]
		} else {
			placeholder = strings.Repeat("*", len(existing))
		}
		description = "Enter a new API key or press Enter to keep existing"
	}

	input := huh.NewInput().
		Title("API Key").
		Description(description).
		Placeholder(placeholder).
		EchoMode(huh.EchoModePassword).
		Value(&apiKey)

	form := huh.NewForm(huh.NewGroup(input)).WithOutput(os.Stderr)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("prompt cancelled")
	}

	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" && existing != "" {
		return existing, nil
	}
	if apiKey == "" {
		return "", fmt.Errorf("API key is required")
	}
	return apiKey, nil
}

func promptModel(existing string) (string, error) {
	defaultModel := config.DefaultModel
	if existing != "" {
		defaultModel = existing
	}

	var model string
	sel := huh.NewSelect[string]().
		Title("Model").
		Description("Choose a default model").
		Options(
			huh.NewOption("Claude Sonnet 4.5 (recommended)", "claude-sonnet-4-5-20250929"),
			huh.NewOption("Claude Haiku 3.5", "claude-haiku-4-5-20251001"),
			huh.NewOption("Claude Opus 4", "claude-opus-4-20250514"),
		).
		Value(&model)

	// Pre-select the existing/default model
	model = defaultModel

	form := huh.NewForm(huh.NewGroup(sel)).WithOutput(os.Stderr)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("prompt cancelled")
	}

	return model, nil
}

func validateAPIKey(ctx context.Context, apiKey string) bool {
	stop := ui.Spinner("Validating API key...")
	defer stop()

	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	_, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_5_20250929,
		MaxTokens: 1,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("hi")),
		},
	})
	return err == nil
}

func promptSaveAnyway() (bool, error) {
	var save bool
	confirm := huh.NewConfirm().
		Title("Save anyway?").
		Description("The API key could not be validated. Save it anyway?").
		Affirmative("Yes").
		Negative("No").
		Value(&save)

	form := huh.NewForm(huh.NewGroup(confirm)).WithOutput(os.Stderr)
	if err := form.Run(); err != nil {
		return false, fmt.Errorf("prompt cancelled")
	}
	return save, nil
}
