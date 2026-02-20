package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_SingleScenario(t *testing.T) {
	content := []byte(`Feature: Login
  Scenario: User logs in
    Given a user
    When  they log in
    Then  they see the dashboard
`)
	doc, errors := Parse("login.ft", content)
	require.Empty(t, errors)
	assert.Equal(t, "Login", doc.Feature.Header.Name)
	require.Len(t, doc.Feature.Scenarios, 1)
	assert.Equal(t, "User logs in", doc.Feature.Scenarios[0].Scenario.Name)
	assert.Equal(t, 2, doc.Feature.Scenarios[0].Line)
}

func TestParse_MultipleScenarios(t *testing.T) {
	content := []byte(`Feature: Login
  Scenario: User logs in
    Given a user

  Scenario: User fails login
    Given a user
`)
	doc, errors := Parse("login.ft", content)
	require.Empty(t, errors)
	require.Len(t, doc.Feature.Scenarios, 2)
	assert.Equal(t, "User logs in", doc.Feature.Scenarios[0].Scenario.Name)
	assert.Equal(t, "User fails login", doc.Feature.Scenarios[1].Scenario.Name)
}

func TestParse_Background(t *testing.T) {
	content := []byte(`Feature: Login
  Background:
    Given a registered user

  Scenario: User logs in
    When  they log in
    Then  they see the dashboard
`)
	doc, errors := Parse("login.ft", content)
	require.Empty(t, errors)
	assert.NotNil(t, doc.Feature.Background)
	require.Len(t, doc.Feature.Scenarios, 1)
	assert.Equal(t, "User logs in", doc.Feature.Scenarios[0].Scenario.Name)
}

func TestParse_ExistingFtTags(t *testing.T) {
	content := []byte(`Feature: Login
  @ft:1
  Scenario: User logs in
    Given a user
`)
	doc, errors := Parse("login.ft", content)
	require.Empty(t, errors)
	require.Len(t, doc.Feature.Scenarios, 1)
	require.Len(t, doc.Feature.Scenarios[0].Tags, 1)
	assert.Equal(t, "@ft:1", doc.Feature.Scenarios[0].Tags[0].Name)
}

func TestParse_ScenarioOutlineError(t *testing.T) {
	content := []byte(`Feature: Login
  Scenario Outline: User logs in
    Given a user
`)
	_, errors := Parse("login.ft", content)
	require.Len(t, errors, 1)
	assert.Equal(t, "Scenario Outline is not supported", errors[0].Message)
	assert.Equal(t, 2, errors[0].Line)
}

func TestParse_RuleError(t *testing.T) {
	content := []byte(`Feature: Login
  Rule: Business rule
    Scenario: Test
`)
	_, errors := Parse("login.ft", content)
	require.Len(t, errors, 1)
	assert.Equal(t, "Rule is not supported", errors[0].Message)
}

func TestParse_ExamplesError(t *testing.T) {
	content := []byte(`Feature: Login
  Examples: Table
    | a |
`)
	_, errors := Parse("login.ft", content)
	require.Len(t, errors, 1)
	assert.Equal(t, "Examples is not supported", errors[0].Message)
}

func TestParse_NoFeatureLine(t *testing.T) {
	content := []byte(`  Scenario: User logs in
    Given a user
`)
	doc, errors := Parse("login.ft", content)
	require.Empty(t, errors)
	assert.Equal(t, "login", doc.Feature.Header.Name)
	require.Len(t, doc.Feature.Scenarios, 1)
	assert.Equal(t, "User logs in", doc.Feature.Scenarios[0].Scenario.Name)
}

func TestParse_BlankLinesWithinScenario(t *testing.T) {
	content := []byte(`Feature: Login
  Scenario: User logs in
    Given a user

    When  they log in

    Then  they see the dashboard
`)
	doc, errors := Parse("login.ft", content)
	require.Empty(t, errors)
	require.Len(t, doc.Feature.Scenarios, 1)
	assert.Equal(t, "User logs in", doc.Feature.Scenarios[0].Scenario.Name)
}

func TestParse_Comments(t *testing.T) {
	content := []byte(`# This is a comment
Feature: Login
  # Another comment
  Scenario: User logs in
    Given a user
`)
	doc, errors := Parse("login.ft", content)
	require.Empty(t, errors)
	assert.Equal(t, "Login", doc.Feature.Header.Name)
	require.Len(t, doc.Feature.Scenarios, 1)
}

func TestParse_MultipleTags(t *testing.T) {
	content := []byte(`Feature: Login
  @smoke @ft:5 @regression
  Scenario: User logs in
    Given a user
`)
	doc, errors := Parse("login.ft", content)
	require.Empty(t, errors)
	require.Len(t, doc.Feature.Scenarios, 1)
	tags := doc.Feature.Scenarios[0].Tags
	require.Len(t, tags, 3)
	assert.Equal(t, "@smoke", tags[0].Name)
	assert.Equal(t, "@ft:5", tags[1].Name)
	assert.Equal(t, "@regression", tags[2].Name)
}

func TestParse_EmptyFile(t *testing.T) {
	content := []byte("")
	doc, errors := Parse("empty.ft", content)
	require.Empty(t, errors)
	assert.Equal(t, "empty", doc.Feature.Header.Name)
}

func TestParse_TagsBeforeMultipleScenarios(t *testing.T) {
	content := []byte(`Feature: Login
  @tag1
  Scenario: First
    Given a

  @tag2
  Scenario: Second
    Given b
`)
	doc, errors := Parse("login.ft", content)
	require.Empty(t, errors)
	require.Len(t, doc.Feature.Scenarios, 2)
	require.Len(t, doc.Feature.Scenarios[0].Tags, 1)
	assert.Equal(t, "@tag1", doc.Feature.Scenarios[0].Tags[0].Name)
	require.Len(t, doc.Feature.Scenarios[1].Tags, 1)
	assert.Equal(t, "@tag2", doc.Feature.Scenarios[1].Tags[0].Name)
}

func TestParse_DocStringContentIsOpaque(t *testing.T) {
	content := []byte(`Feature: Parse Scenarios
  Scenario: Already-tagged scenario is skipped
    Given the file fts/login.ft contains:
      """
      Feature: Login
        @ft:1
        Scenario: User logs in
          Given a user
      """
    When the user runs sync
    Then no new scenarios record is created
`)
	doc, errors := Parse("test.ft", content)
	require.Empty(t, errors)
	require.Len(t, doc.Feature.Scenarios, 1)
	assert.Equal(t, "Already-tagged scenario is skipped", doc.Feature.Scenarios[0].Scenario.Name)
}

func TestParse_DocStringWithBackticks(t *testing.T) {
	content := []byte("Feature: Test\n  Scenario: Has code block\n    Given content:\n      ```\n      Scenario: Not real\n      @ft:99\n      ```\n    Then it works\n")
	doc, errors := Parse("test.ft", content)
	require.Empty(t, errors)
	require.Len(t, doc.Feature.Scenarios, 1)
	assert.Equal(t, "Has code block", doc.Feature.Scenarios[0].Scenario.Name)
}

func TestParse_MultipleScenarios_WithDocStrings(t *testing.T) {
	content := []byte(`Feature: Phase 3
  Scenario: First
    Given file contains:
      """
      Feature: Login
        Scenario: Inner
      """
    Then it works

  Scenario: Second
    Given something
`)
	doc, errors := Parse("test.ft", content)
	require.Empty(t, errors)
	require.Len(t, doc.Feature.Scenarios, 2)
	assert.Equal(t, "First", doc.Feature.Scenarios[0].Scenario.Name)
	assert.Equal(t, "Second", doc.Feature.Scenarios[1].Scenario.Name)
}
