package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	face "github.com/benaskins/axon-face"
	loop "github.com/benaskins/axon-loop"
	"github.com/benaskins/axon-talk/anthropic"
	"github.com/benaskins/axon-talk/openai"

	"github.com/benaskins/vita/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Read LinkedIn text from stdin or file argument
	linkedinText, err := readInput()
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	if linkedinText == "" {
		return fmt.Errorf("no input provided — pipe LinkedIn profile text via stdin or pass a file path as argument\n\nUsage:\n  pbpaste | vita\n  vita profile.txt")
	}

	// Setup logging
	home, _ := os.UserHomeDir()
	cleanup, err := face.SetupLogging(home + "/.local/share/vita/logs")
	if err != nil {
		return fmt.Errorf("setup logging: %w", err)
	}
	defer cleanup()

	// Select LLM client
	client, modelName := selectClient()

	model := tui.New(client, modelName, nil, linkedinText)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}

func readInput() (string, error) {
	// Check for file argument
	if len(os.Args) > 1 {
		data, err := os.ReadFile(filepath.Clean(os.Args[1]))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	// Check for stdin
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", nil
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return "", nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func selectClient() (loop.LLMClient, string) {
	// Try Anthropic first
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey != "" {
		slog.Info("using Anthropic API")
		client := anthropic.NewClient("https://api.anthropic.com", apiKey)
		return client, envOrDefault("VITA_MODEL", "claude-sonnet-4-6")
	}

	// Fall back to local OpenAI-compatible server (llama-server, vllm-mlx, etc.)
	baseURL := envOrDefault("OPENAI_BASE_URL", "http://localhost:8080")
	client := openai.NewClient(baseURL, "")
	slog.Info("using OpenAI-compatible server", "url", baseURL)
	return client, envOrDefault("VITA_MODEL", "qwen3:32b")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
