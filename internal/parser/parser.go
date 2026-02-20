package parser

import (
	"regexp"
	"strings"
)

var tagPattern = regexp.MustCompile(`@[^@\s]+`)

// Parse parses a .ft file and returns a Document AST and any parse errors.
func Parse(filename string, content []byte) (*Document, []ParseError) {
	lines := strings.Split(string(content), "\n")
	var errors []ParseError

	doc := &Document{}
	feature := &Feature{}
	doc.Feature = feature

	i := 0

	// Skip leading blanks and comments
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}
		break
	}

	// Collect feature-level tags
	var featureTags []Tag
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "@") {
			featureTags = append(featureTags, parseTags(trimmed)...)
			i++
			continue
		}
		break
	}

	// Look for Feature: line
	if i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "Feature:") {
			feature.Header.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "Feature:"))
			feature.Header.Tags = featureTags
			i++

			// Scan description lines until keyword or tag
			var descLines []string
			for i < len(lines) {
				trimmed := strings.TrimSpace(lines[i])
				if isKeyword(trimmed) || isTagLine(trimmed) {
					break
				}
				descLines = append(descLines, lines[i])
				i++
			}
			if len(descLines) > 0 {
				feature.Header.Description = strings.Join(descLines, "\n")
			}
		} else {
			// No Feature: line — use filename without extension
			name := filename
			if idx := strings.LastIndex(name, "/"); idx >= 0 {
				name = name[idx+1:]
			}
			if idx := strings.LastIndex(name, "."); idx >= 0 {
				name = name[:idx]
			}
			feature.Header.Name = name
			feature.Header.Tags = featureTags
		}
	} else {
		// Empty file after blanks/comments
		name := filename
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			name = name[:idx]
		}
		feature.Header.Name = name
		feature.Header.Tags = featureTags
	}

	// Body loop
	var pendingTags []Tag
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])

		// Skip doc strings
		if isDocStringDelimiter(trimmed) {
			i = skipDocString(lines, i)
			continue
		}

		// Skip blank lines and comments in body
		if trimmed == "" {
			i++
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}

		// Tag line
		if isTagLine(trimmed) {
			pendingTags = append(pendingTags, parseTags(trimmed)...)
			i++
			continue
		}

		// Background:
		if strings.HasPrefix(trimmed, "Background:") {
			pendingTags = nil // Background doesn't get tags
			bg := &Background{}
			i++
			i = consumeBlock(lines, i)
			feature.Background = bg
			continue
		}

		// Scenario:
		if strings.HasPrefix(trimmed, "Scenario:") {
			scenarioLine := i + 1 // 1-based
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "Scenario:"))
			sd := ScenarioDefinition{
				Tags:     pendingTags,
				Scenario: Scenario{Name: name},
				Line:     scenarioLine,
			}
			pendingTags = nil
			i++
			// Consume until next Scenario:, Background:, tag line preceding Scenario:, or EOF
			for i < len(lines) {
				t := strings.TrimSpace(lines[i])
				// Skip over doc strings entirely
				if isDocStringDelimiter(t) {
					i = skipDocString(lines, i)
					continue
				}
				if strings.HasPrefix(t, "Scenario:") || strings.HasPrefix(t, "Background:") {
					break
				}
				if strings.HasPrefix(t, "Scenario Outline:") || strings.HasPrefix(t, "Rule:") || strings.HasPrefix(t, "Examples:") {
					break
				}
				if isTagLine(t) {
					if tagPrecedesKeyword(lines, i) {
						break
					}
				}
				i++
			}
			feature.Scenarios = append(feature.Scenarios, sd)
			continue
		}

		// Unsupported keywords
		if strings.HasPrefix(trimmed, "Scenario Outline:") {
			errors = append(errors, ParseError{Line: i + 1, Message: "Scenario Outline is not supported"})
			i++
			i = consumeBlock(lines, i)
			continue
		}
		if strings.HasPrefix(trimmed, "Rule:") {
			errors = append(errors, ParseError{Line: i + 1, Message: "Rule is not supported"})
			i++
			i = consumeBlock(lines, i)
			continue
		}
		if strings.HasPrefix(trimmed, "Examples:") {
			errors = append(errors, ParseError{Line: i + 1, Message: "Examples is not supported"})
			i++
			i = consumeBlock(lines, i)
			continue
		}

		// Otherwise — content line for current block, skip
		i++
	}

	return doc, errors
}

func parseTags(line string) []Tag {
	matches := tagPattern.FindAllString(line, -1)
	var tags []Tag
	for _, m := range matches {
		tags = append(tags, Tag{Name: m})
	}
	return tags
}

func isTagLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, "@")
}

func isKeyword(trimmed string) bool {
	return strings.HasPrefix(trimmed, "Feature:") ||
		strings.HasPrefix(trimmed, "Background:") ||
		strings.HasPrefix(trimmed, "Scenario:") ||
		strings.HasPrefix(trimmed, "Scenario Outline:") ||
		strings.HasPrefix(trimmed, "Rule:") ||
		strings.HasPrefix(trimmed, "Examples:")
}

func isDocStringDelimiter(trimmed string) bool {
	return strings.HasPrefix(trimmed, `"""`) || strings.HasPrefix(trimmed, "```")
}

// skipDocString advances past a doc string block. i points at the opening delimiter.
// Returns the index of the line after the closing delimiter.
func skipDocString(lines []string, i int) int {
	opener := strings.TrimSpace(lines[i])
	delimiter := `"""`
	if strings.HasPrefix(opener, "```") {
		delimiter = "```"
	}
	i++ // move past opening delimiter
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == delimiter {
			return i + 1 // past the closing delimiter
		}
		i++
	}
	return i // EOF without closing delimiter
}

// consumeBlock advances past content lines, skipping over doc strings,
// until the next keyword, tag line, or EOF.
func consumeBlock(lines []string, i int) int {
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if isDocStringDelimiter(t) {
			i = skipDocString(lines, i)
			continue
		}
		if isKeyword(t) || isTagLine(t) {
			break
		}
		i++
	}
	return i
}

// tagPrecedesKeyword checks if a tag line at index i is followed by a Scenario: or keyword line.
func tagPrecedesKeyword(lines []string, i int) bool {
	for j := i + 1; j < len(lines); j++ {
		t := strings.TrimSpace(lines[j])
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasPrefix(t, "@") {
			continue
		}
		return strings.HasPrefix(t, "Scenario:") ||
			strings.HasPrefix(t, "Background:") ||
			strings.HasPrefix(t, "Scenario Outline:") ||
			strings.HasPrefix(t, "Rule:") ||
			strings.HasPrefix(t, "Examples:")
	}
	return false
}
