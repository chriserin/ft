package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransform_FeatureName(t *testing.T) {
	content := []byte(`Feature: Login
  Scenario: User logs in
    Given a user
`)
	doc, errors := Parse("login.ft", content)
	pf := Transform(doc, "login.ft", content, errors)

	assert.Equal(t, "Login", pf.Name)
}

func TestTransform_FtTagParsing(t *testing.T) {
	content := []byte(`Feature: Login
  @ft:42
  Scenario: User logs in
    Given a user
`)
	doc, errors := Parse("login.ft", content)
	pf := Transform(doc, "login.ft", content, errors)

	require.Len(t, pf.Scenarios, 1)
	assert.Equal(t, "42", pf.Scenarios[0].FtTag)
	assert.Equal(t, "User logs in", pf.Scenarios[0].Name)
}

func TestTransform_OtherTags(t *testing.T) {
	content := []byte(`Feature: Login
  @smoke @ft:1 @regression
  Scenario: User logs in
    Given a user
`)
	doc, errors := Parse("login.ft", content)
	pf := Transform(doc, "login.ft", content, errors)

	require.Len(t, pf.Scenarios, 1)
	assert.Equal(t, "1", pf.Scenarios[0].FtTag)
	assert.Equal(t, []string{"@smoke", "@regression"}, pf.Scenarios[0].OtherTags)
}

func TestTransform_ContentCapture(t *testing.T) {
	content := []byte(`Feature: Login
  Scenario: User logs in
    Given a user
    When  they log in
    Then  they see the dashboard`)
	doc, errors := Parse("login.ft", content)
	pf := Transform(doc, "login.ft", content, errors)

	require.Len(t, pf.Scenarios, 1)
	expected := `  Scenario: User logs in
    Given a user
    When  they log in
    Then  they see the dashboard`
	assert.Equal(t, expected, pf.Scenarios[0].Content)
}

func TestTransform_MultipleScenarios_ContentCapture(t *testing.T) {
	content := []byte(`Feature: Login
  Scenario: User logs in
    Given a user

  Scenario: User fails login
    Given a bad password`)
	doc, errors := Parse("login.ft", content)
	pf := Transform(doc, "login.ft", content, errors)

	require.Len(t, pf.Scenarios, 2)
	assert.Contains(t, pf.Scenarios[0].Content, "User logs in")
	assert.Contains(t, pf.Scenarios[1].Content, "User fails login")
}

func TestTransform_LineNumbers(t *testing.T) {
	content := []byte(`Feature: Login

  Scenario: User logs in
    Given a user
`)
	doc, errors := Parse("login.ft", content)
	pf := Transform(doc, "login.ft", content, errors)

	require.Len(t, pf.Scenarios, 1)
	assert.Equal(t, 3, pf.Scenarios[0].Line)
}

func TestTransform_Errors(t *testing.T) {
	content := []byte(`Feature: Login
  Scenario Outline: User logs in
    Given a user
`)
	doc, errors := Parse("login.ft", content)
	pf := Transform(doc, "login.ft", content, errors)

	require.Len(t, pf.Errors, 1)
	assert.Equal(t, "Scenario Outline is not supported", pf.Errors[0].Message)
}

func TestTransform_NoFeatureLine(t *testing.T) {
	content := []byte(`  Scenario: User logs in
    Given a user
`)
	doc, errors := Parse("fts/login.ft", content)
	pf := Transform(doc, "fts/login.ft", content, errors)

	assert.Equal(t, "login", pf.Name)
}
