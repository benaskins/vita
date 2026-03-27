---
module: github.com/benaskins/vita
kind: app
---

# vita

Bubble Tea CLI for structured interviews. Interview and draft phases. Early stage.

## Build & Test

```bash
go test ./...
go vet ./...
```

## Structure

```
cmd/vita/main.go        entry point
internal/tui/           Bubble Tea model, interview and draft phases
```

## Key dependencies

- axon-face (reusable Bubble Tea chat component)
- axon-loop (conversation loop + streaming + tool dispatch)
- axon-talk (LLM provider adapters)
- axon-tool (tool definitions)
- bubbletea, lipgloss, bubbles (TUI)

All axon modules use local `replace` directives during development.
