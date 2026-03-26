package tui

import "strings"

// splitSections splits markdown content by headings.
// Each section includes the heading and all content until the next
// heading of the same or higher level.
func splitSections(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var current strings.Builder
	inSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") && inSection {
			sections = append(sections, strings.TrimSpace(current.String()))
			current.Reset()
		}
		if strings.HasPrefix(trimmed, "#") {
			inSection = true
		}
		if inSection {
			current.WriteString(line)
			current.WriteString("\n")
		}
	}

	if current.Len() > 0 {
		sections = append(sections, strings.TrimSpace(current.String()))
	}

	// If no headings found, treat entire content as one section
	if len(sections) == 0 && strings.TrimSpace(content) != "" {
		sections = append(sections, strings.TrimSpace(content))
	}

	return sections
}

// assembleSections joins approved sections back into a single document.
func assembleSections(sections []string) string {
	return strings.Join(sections, "\n\n")
}
