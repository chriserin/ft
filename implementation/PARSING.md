# `ft` — Gherkin Parsing

Go parser for `.ft` files, modeled after the tree-sitter Gherkin grammar at `~/projects/features/ts/tree-sitter-gherkin-binhtddev`.

Supports a subset of Gherkin. English keywords only.

---

## Supported Keywords

- `Feature:`
- `Background:`
- `Scenario:`
- `Given`, `When`, `Then`, `And`, `But`, `*` (step keywords)

## Unsupported Keywords (syntax errors)

- `Scenario Outline:`
- `Rule:`
- `Examples:`

## File Structure

```
Feature: <name>
  <description>

  Background:
    <steps>

  @tag1 @tag2
  Scenario: <name>
    <description>
    <steps>

  @tag1
  Scenario: <name>
    <steps>
```

## Parsing Rules

### Feature

- First non-comment, non-blank line must be `Feature:` followed by a name
- If no `Feature:` line is found, the filename (without extension) is used as the name
- Description lines follow the `Feature:` line until the first keyword or tag

### Background

- Starts with `Background:` keyword
- Optional description lines after the keyword line
- Contains steps
- Ends at the next `Scenario:`, tag line, or EOF
- At most one `Background:` per file

### Scenario

- Starts with `Scenario:` keyword
- Optional description lines after the keyword line
- Contains steps
- Ends at the next `Scenario:`, `Background:`, tag line (preceding a `Scenario:`), or EOF
- A scenario does NOT end at a blank line — blank lines within a scenario are allowed

### Tags

- Lines starting with `@` (after optional whitespace)
- Multiple tags on one line: `@tag1 @tag2 @tag3`
- Pattern: `@[^@\s]+`
- Tags attach to the immediately following `Feature:` or `Scenario:` block
- The `@ft:<id>` tag is always the first tag on the line immediately above `Scenario:`

### Steps

- Start with a step keyword: `Given`, `When`, `Then`, `And`, `But`, or `*`
- The keyword is followed by text on the same line
- A step may have a step argument on the following lines (doc string or data table)

### Doc Strings

- Delimited by `"""` or `` ``` `` on their own lines
- Content between delimiters is preserved as-is (including blank lines and indentation)
- Optional media type on the opening delimiter line: `"""json`
- Escape sequences within: `\\`, `\"`

### Data Tables

- Rows of `|`-delimited cells
- First row is treated as the header
- Cell content is trimmed of surrounding whitespace
- Escape sequences within cells: `\\`, `\|`, `\n`
- Empty cells are allowed: `||`

### Comments

- Lines starting with `#` (after optional whitespace)
- Can appear anywhere: between scenarios, between steps, after tags
- Comments are preserved but not part of the parsed structure
- System error comments (`# ft error:`) are written to the top of the file

---

## Architecture: Two-Layer Design

The parser is split into two layers to enable direct comparison with the tree-sitter grammar.

### Layer 1: Tree-sitter-compatible AST

Go structs that mirror tree-sitter node types 1:1. The parser produces the same structural output as tree-sitter, making test comparison trivial.

```go
type Document struct {
    Feature *Feature
}

type Feature struct {
    Header     FeatureHeader
    Background *Background
    Scenarios  []ScenarioDefinition
}

type FeatureHeader struct {
    Tags        []Tag
    Name        string
    Description string
}

type Background struct {
    Description string
    StepGroups  []StepGroup
}

type ScenarioDefinition struct {
    Tags     []Tag
    Scenario Scenario
}

type Scenario struct {
    Name        string
    Description string
    StepGroups  []StepGroup
}

type Tag struct {
    Name string // e.g. "@smoke", "@ft:42"
}

type StepGroup struct {
    Step     Step
    AltSteps []Step // And, But, *
}

type Step struct {
    Keyword  string // Given, When, Then, And, But, *
    Text     string
    Argument *StepArgument
}

type StepArgument struct {
    DocString *DocString
    DataTable *DataTable
}

type DocString struct {
    MediaType string
    Content   string
}

type DataTable struct {
    HeaderRow []string
    Rows      [][]string
}

type ParseError struct {
    Line    int
    Message string
}
```

### Layer 2: Application Model

A thin transformation on top of the AST that extracts what `ft` needs. This layer is not part of the parser — it consumes the AST.

```go
type ParsedFile struct {
    Name       string
    Background *Background
    Scenarios  []ParsedScenario
    Errors     []ParseError
}

type ParsedScenario struct {
    Name       string   // from Scenario: line
    FtTag      string   // @ft:<id> if present
    OtherTags  []string // non-@ft tags
    Content    string   // raw gherkin text for DB storage / rehydration
    LineNumber int      // line number of the Scenario: line
}
```

The `Content` field captures the raw text from the `Scenario:` line through to the end of the scenario. It does NOT include the `@ft:` tag line — that is written separately during rehydration.

---

## Testing: Tree-sitter Corpus Integration

The Go parser is tested against the tree-sitter grammar's test corpus at `test/corpus/`.

### Corpus Format

Each test case in the corpus follows this format:

```
===
Test Name
===
<input gherkin>
---
<expected parse tree as S-expression>
```

### Test Strategy

1. Parse the corpus files to extract test cases
2. Parse the S-expression into the Layer 1 Go structs
3. For each test case:
   - If the expected tree contains only supported node types — run the Go parser on the input, compare the AST against the S-expression-derived structs
   - If the expected tree contains unsupported node types (`scenario_outline_line`, `rule`, `examples`) — verify the Go parser returns a syntax error
   - Skip i18n tests (English only)

Because the Go structs mirror tree-sitter node types 1:1, comparison is direct — no translation layer needed.

### S-Expression Parser

A small Go utility parses tree-sitter S-expressions into the same AST structs:

```go
type SNode struct {
    Type     string
    Children []SNode
}

func ParseSExpression(input string) SNode { ... }
func SNodeToDocument(node SNode) Document { ... }
```

This enables the test suite to be automatically derived from the tree-sitter corpus. When the corpus is updated, Go tests reflect the changes.

---

## Error Handling

When a syntax error is encountered:
1. Record the error with line number and message
2. Continue parsing the rest of the file (best-effort)
3. Return all errors in the parse result
4. The caller writes `# ft error: <message> (line <n>)` to the top of the file
