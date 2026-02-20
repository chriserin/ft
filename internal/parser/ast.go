package parser

// Layer 1: Tree-sitter-compatible AST types

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
	Line     int // 1-based line number of Scenario: line
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
