package parser

import (
	"strings"
)

// ParsedFile is the Layer 2 application model extracted from the AST.
type ParsedFile struct {
	Name      string
	Scenarios []ParsedScenario
	Errors    []ParseError
}

// ParsedScenario represents a single scenario extracted from a .ft file.
type ParsedScenario struct {
	Name      string   // from Scenario: line
	FtTag     string   // just the ID portion, e.g. "1"
	OtherTags []string // non-@ft tags
	Content   string   // raw text from Scenario: line to end of scenario
	Line      int      // 1-based line number of Scenario: line
}

// Transform converts a Layer 1 Document into a Layer 2 ParsedFile.
func Transform(doc *Document, filename string, content []byte, errors []ParseError) *ParsedFile {
	pf := &ParsedFile{
		Errors: errors,
	}

	if doc.Feature != nil {
		pf.Name = doc.Feature.Header.Name
	} else {
		pf.Name = filenameWithoutExt(filename)
	}

	if doc.Feature == nil {
		return pf
	}

	lines := strings.Split(string(content), "\n")

	for _, sd := range doc.Feature.Scenarios {
		ps := ParsedScenario{
			Name: sd.Scenario.Name,
			Line: sd.Line,
		}

		// Partition tags into FtTag vs OtherTags
		for _, tag := range sd.Tags {
			if strings.HasPrefix(tag.Name, "@ft:") {
				ps.FtTag = strings.TrimPrefix(tag.Name, "@ft:")
			} else {
				ps.OtherTags = append(ps.OtherTags, tag.Name)
			}
		}

		// Extract content: from Scenario: line to end of scenario
		startLine := sd.Line - 1 // 0-based
		endLine := len(lines)

		// Find the next scenario's start line or use end of file
		for _, other := range doc.Feature.Scenarios {
			if other.Line > sd.Line && other.Line-1 < endLine {
				// The content ends before the next scenario's tags or Scenario: line
				candidateEnd := other.Line - 1 // 0-based index of next Scenario: line
				// Walk back to exclude tag lines and blank lines before the next scenario
				for candidateEnd > startLine {
					t := strings.TrimSpace(lines[candidateEnd-1])
					if t == "" || strings.HasPrefix(t, "@") || strings.HasPrefix(t, "#") {
						candidateEnd--
					} else {
						break
					}
				}
				if candidateEnd < endLine {
					endLine = candidateEnd
				}
			}
		}

		// Also check Background position
		if doc.Feature.Background != nil {
			// Background doesn't have a Line field in our AST, so we skip this check
		}

		// Trim trailing blank lines
		for endLine > startLine && strings.TrimSpace(lines[endLine-1]) == "" {
			endLine--
		}

		if startLine < len(lines) {
			contentLines := lines[startLine:endLine]
			ps.Content = strings.Join(contentLines, "\n")
		}

		pf.Scenarios = append(pf.Scenarios, ps)
	}

	return pf
}

func filenameWithoutExt(filename string) string {
	name := filename
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[:idx]
	}
	return name
}
