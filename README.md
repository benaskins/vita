# vita

Structured interview generator from LinkedIn profiles. Reads profile text,
conducts an LLM-powered interview via Bubble Tea TUI, and drafts output.

## Usage

```bash
# From a file
vita --file profile.txt

# From stdin
pbpaste | vita
```

## How it works

1. Provide LinkedIn profile text as input
2. vita enters an interview phase where the LLM asks structured questions
3. Your answers are collected via the TUI
4. A draft is generated from the interview

## Architecture

Bubble Tea TUI with two phases: interview and draft. Built on axon-face
for the chat component and axon-loop for the conversation engine.

## Dependencies

- axon-face (TUI components)
- axon-loop, axon-talk, axon-tool (LLM primitives)

## Build & Test

```bash
go test ./...
go vet ./...
just build && just install
```
