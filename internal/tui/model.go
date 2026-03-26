// Package tui provides the Bubble Tea terminal UI for vita.
package tui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	face "github.com/benaskins/axon-face"
	loop "github.com/benaskins/axon-loop"
	tool "github.com/benaskins/axon-tool"
)

// phase represents the current application phase.
type phase int

const (
	phaseInterview phase = iota
	phaseDraft
)

// phaseSwitchMsg triggers a transition to draft phase.
type phaseSwitchMsg struct{}

// sectionReviseMsg carries a revised section from the LLM.
type sectionReviseMsg struct {
	content string
	err     error
}

// Model is the top-level Bubble Tea model for vita.
type Model struct {
	face.Chat

	phase  phase
	client loop.LLMClient
	tools  map[string]tool.ToolDef
	model  string // LLM model name

	// Draft state
	sections     []string
	sectionIndex int
	approved     []bool
	fullDraft    string

	// Session
	session    *face.Session
	sessionDir string
}

// New creates a new vita Model.
func New(client loop.LLMClient, modelName string, tools map[string]tool.ToolDef, linkedinText string) Model {
	chat := face.New("vita")
	chat.Messages = []loop.Message{
		{Role: loop.RoleSystem, Content: systemPrompt(linkedinText)},
	}

	home, _ := os.UserHomeDir()

	return Model{
		Chat:       chat,
		phase:      phaseInterview,
		client:     client,
		tools:      tools,
		model:      modelName,
		sessionDir: filepath.Join(home, ".local", "share", "vita", "sessions"),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.Chat.InitCmd(),
		m.startLLM(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.phase {
	case phaseInterview:
		return m.updateInterview(msg)
	case phaseDraft:
		return m.updateDraft(msg)
	}
	return m, nil
}

func (m Model) View() string {
	switch m.phase {
	case phaseInterview:
		return m.viewInterview()
	case phaseDraft:
		return m.viewDraft()
	}
	return ""
}

// --- Interview phase ---

func (m Model) updateInterview(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Check for /draft command before base handling
		if msg.String() == "enter" && !m.Waiting {
			text := strings.TrimSpace(m.Input.Value())
			if text == "/draft" {
				m.Input.Reset()
				return m, func() tea.Msg { return phaseSwitchMsg{} }
			}
		}

		cmd, handled := m.Chat.HandleKey(msg)
		if handled {
			if cmd != nil {
				return m, cmd
			}
			// enter was handled — start the stream
			if msg.String() == "enter" && m.Waiting {
				m.saveSession()
				return m, m.startLLM()
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.Chat.HandleResize(msg)
		return m, nil

	case face.StreamTickMsg:
		cmd := m.Chat.HandleStreamTick(msg)
		if cmd == nil {
			// Stream done — save session
			m.saveSession()
		}
		return m, cmd

	case phaseSwitchMsg:
		slog.Info("phase transition", "from", "interview", "to", "draft")
		return m.transitionToDraft()
	}

	cmd := m.Chat.UpdateInput(msg)
	return m, cmd
}

func (m Model) viewInterview() string {
	model := m.Styles.Model.Render(m.model)
	status := m.Styles.Status.Render("ctrl+c quit | /draft to start drafting") + "  " + model
	if m.Waiting {
		status = m.Styles.Status.Render("thinking...") + "  " + model
	}
	return m.Chat.View(status)
}

// --- Draft phase ---

func (m Model) transitionToDraft() (tea.Model, tea.Cmd) {
	m.phase = phaseDraft
	m.Chat.AppendEntry(face.Entry{Role: face.RoleAgent, Content: "Generating your resume draft..."})

	// Add draft prompt to messages
	m.Messages = append(m.Messages, loop.Message{Role: loop.RoleUser, Content: DraftPrompt})

	// Stream the draft
	return m, m.startDraftLLM()
}

func (m Model) startDraftLLM() tea.Cmd {
	messages := make([]loop.Message, len(m.Messages))
	copy(messages, m.Messages)

	req := &loop.Request{
		Model:    m.model,
		Messages: messages,
		Stream:   true,
	}

	cfg := loop.RunConfig{
		Client:  m.client,
		Request: req,
	}

	ch := loop.Stream(context.Background(), cfg)
	return face.WaitForEvent(ch)
}

func (m Model) updateDraft(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.Waiting {
				return m, nil
			}
			text := strings.TrimSpace(m.Input.Value())
			if text == "" {
				return m, nil
			}
			m.Input.Reset()

			if text == "/keep" || text == "/k" || text == "k" || text == "keep" {
				return m.approveSection()
			}
			if text == "/done" {
				return m.saveDraft()
			}

			// Revision feedback
			m.Chat.AppendEntry(face.Entry{Role: face.RoleUser, Content: text})
			m.Waiting = true
			return m, m.reviseSection(text)
		}

	case tea.WindowSizeMsg:
		m.Chat.HandleResize(msg)
		return m, nil

	case face.StreamTickMsg:
		ev := msg.Event
		if ev.Done {
			content := ev.Content
			if content == "" {
				content = m.Streaming
			}
			// Parse into sections
			m.sections = splitSections(content)
			m.approved = make([]bool, len(m.sections))
			m.sectionIndex = 0
			m.fullDraft = content
			m.Streaming = ""
			m.Waiting = false
			m.Messages = append(m.Messages, loop.Message{Role: loop.RoleAssistant, Content: content})
			m.showCurrentSection()
			return m, nil
		}
		if ev.Token != "" {
			m.Streaming += ev.Token
			m.RefreshViewport()
		}
		if ev.Err != nil {
			m.Chat.AppendEntry(face.Entry{Role: face.RoleAgent, Content: fmt.Sprintf("Error: %v", ev.Err)})
			m.Waiting = false
			return m, nil
		}
		return m, face.WaitForEvent(msg.Ch)

	case sectionReviseMsg:
		m.Waiting = false
		if msg.err != nil {
			m.Chat.AppendEntry(face.Entry{Role: face.RoleAgent, Content: fmt.Sprintf("Error: %v", msg.err)})
			return m, nil
		}
		m.sections[m.sectionIndex] = msg.content
		m.showCurrentSection()
		return m, nil
	}

	cmd := m.Chat.UpdateInput(msg)
	return m, cmd
}

func (m *Model) showCurrentSection() {
	// Clear entries and show draft status
	m.Entries = nil

	// Section overview
	var overview strings.Builder
	for i, s := range m.sections {
		marker := "[ ]"
		if m.approved[i] {
			marker = "[✓]"
		}
		if i == m.sectionIndex {
			marker = "[>]"
		}
		// First line of section as label
		label := strings.SplitN(s, "\n", 2)[0]
		overview.WriteString(fmt.Sprintf("%s %d. %s\n", marker, i+1, label))
	}
	m.Chat.AppendEntry(face.Entry{
		Role:    face.RoleAgent,
		Content: fmt.Sprintf("Draft review — section %d of %d\n\n%s\n---\n\n%s\n\n/keep to approve, or type revision feedback", m.sectionIndex+1, len(m.sections), overview.String(), m.sections[m.sectionIndex]),
	})
}

func (m Model) approveSection() (tea.Model, tea.Cmd) {
	m.approved[m.sectionIndex] = true
	m.Chat.AppendEntry(face.Entry{
		Role:    face.RoleAction,
		Content: m.Styles.Approved.Render(fmt.Sprintf("Section %d approved", m.sectionIndex+1)),
	})

	// Find next unapproved
	next := -1
	for i := m.sectionIndex + 1; i < len(m.sections); i++ {
		if !m.approved[i] {
			next = i
			break
		}
	}

	if next == -1 {
		// All approved — save
		return m.saveDraft()
	}

	m.sectionIndex = next
	m.showCurrentSection()
	return m, nil
}

func (m Model) reviseSection(feedback string) tea.Cmd {
	// Build interview transcript from messages
	var transcript strings.Builder
	for _, msg := range m.Messages {
		if msg.Role == loop.RoleUser || msg.Role == loop.RoleAssistant {
			transcript.WriteString(fmt.Sprintf("[%s]: %s\n\n", msg.Role, msg.Content))
		}
	}

	prompt := fmt.Sprintf(RevisionPrompt, transcript.String(), m.fullDraft, m.sections[m.sectionIndex])

	messages := []loop.Message{
		{Role: loop.RoleSystem, Content: prompt},
		{Role: loop.RoleUser, Content: feedback},
	}

	req := &loop.Request{
		Model:    m.model,
		Messages: messages,
		Stream:   false,
	}

	cfg := loop.RunConfig{
		Client:  m.client,
		Request: req,
	}

	return func() tea.Msg {
		result, err := loop.Run(context.Background(), cfg)
		if err != nil {
			return sectionReviseMsg{err: err}
		}
		return sectionReviseMsg{content: result.Content}
	}
}

func (m Model) saveDraft() (tea.Model, tea.Cmd) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, "Documents", "vita")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.Chat.AppendEntry(face.Entry{Role: face.RoleAgent, Content: fmt.Sprintf("Error creating output dir: %v", err)})
		return m, nil
	}

	content := assembleSections(m.sections)

	// Generate filename from first heading or timestamp
	name := "resume"
	for _, s := range m.sections {
		if strings.HasPrefix(s, "# ") {
			name = strings.TrimSpace(strings.TrimPrefix(strings.SplitN(s, "\n", 2)[0], "# "))
			name = strings.ReplaceAll(strings.ToLower(name), " ", "-")
			break
		}
	}

	path := filepath.Join(dir, name+".md")
	// Avoid overwriting
	for i := 2; fileExists(path); i++ {
		path = filepath.Join(dir, fmt.Sprintf("%s-%d.md", name, i))
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		m.Chat.AppendEntry(face.Entry{Role: face.RoleAgent, Content: fmt.Sprintf("Error writing file: %v", err)})
		return m, nil
	}

	if m.session != nil {
		m.session.MarkComplete(m.sessionDir)
	}

	m.Chat.AppendEntry(face.Entry{Role: face.RoleAgent, Content: fmt.Sprintf("Resume saved to %s", path)})
	return m, tea.Quit
}

func (m *Model) saveSession() {
	if m.session == nil {
		m.session = face.NewSession()
	}
	m.session.Messages = m.Messages
	m.session.Phase = "interview"
	if m.phase == phaseDraft {
		m.session.Phase = "draft"
	}
	if err := m.session.Save(m.sessionDir); err != nil {
		slog.Error("failed to save session", "error", err)
	}
}

func (m Model) startLLM() tea.Cmd {
	req := &loop.Request{
		Model: m.model,
	}
	return m.Chat.StartStream(m.client, req, m.tools)
}

func (m Model) viewDraft() string {
	model := m.Styles.Model.Render(m.model)
	status := m.Styles.Status.Render("/keep approve | /done save | ctrl+c quit") + "  " + model
	if m.Waiting {
		status = m.Styles.Status.Render("thinking...") + "  " + model
	}
	return m.Chat.View(status)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
