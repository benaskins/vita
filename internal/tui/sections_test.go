package tui

import "testing"

func TestSplitSections(t *testing.T) {
	input := `# Benjamin Askins
Sydney, Australia

## Summary
Experienced engineering leader.

## Experience
### Head of Data Infrastructure — Block
Jan 2026 - Mar 2026
- Led data platform team

### Director of Engineering — Block
Nov 2024 - Jan 2026
- Built platform foundations

## Skills
Go, Infrastructure, Leadership

## Education
B. Math, University of Wollongong`

	sections := splitSections(input)
	if len(sections) != 7 {
		t.Fatalf("expected 7 sections, got %d", len(sections))
	}
	if sections[0] != "# Benjamin Askins\nSydney, Australia" {
		t.Errorf("section 0 = %q", sections[0])
	}
	if sections[1] != "## Summary\nExperienced engineering leader." {
		t.Errorf("section 1 = %q", sections[1])
	}
}

func TestSplitSectionsNoHeadings(t *testing.T) {
	input := "Just some plain text without headings"
	sections := splitSections(input)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
}

func TestAssembleSections(t *testing.T) {
	sections := []string{"# Name", "## Summary\nText", "## Skills\nGo"}
	result := assembleSections(sections)
	if result != "# Name\n\n## Summary\nText\n\n## Skills\nGo" {
		t.Errorf("unexpected result: %q", result)
	}
}
