@AGENTS.md

## Conventions
- Bubble Tea model architecture — TUI code in internal/tui/
- face.Chat handles the reusable chat component (from axon-face)
- Early stage — single commit, no GitHub release yet

## Constraints
- Composition root — assembles axon-face, axon-loop, axon-talk, axon-tool
- Uses local replace directives during development (managed by `lamina apps wire`)
- Do not add HTTP server code — this is a CLI app, not a service
- Do not import axon (server toolkit) directly
- No justfile yet — use go commands directly

## Testing
- `go test ./...`
