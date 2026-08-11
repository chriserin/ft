package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runAgentInstructions(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, RunAgentInstructions(&buf))
	return buf.String()
}

// @ft:228
func TestAgentInstructions_PrintsBuiltInDefault(t *testing.T) {
	inTempDir(t)
	runInit(t)

	out := runAgentInstructions(t)

	assert.Equal(t, agentInstructionsText, out)
}

// @ft:229
func TestAgentInstructions_WorksWithoutInit(t *testing.T) {
	inTempDir(t)

	out := runAgentInstructions(t)

	assert.Equal(t, agentInstructionsText, out)
}

// @ft:230
// These checks are deliberately light-touch: cmd/agent_instructions.md is
// expected to be hand-edited and refined over time, so we check for stable
// concepts (terms, status names) rather than exact prose.
func TestAgentInstructions_MentionsCoreConcepts(t *testing.T) {
	out := runAgentInstructions(t)

	assert.Contains(t, out, "@ft:<id>")
	assert.Contains(t, out, "ft sync")
	assert.Contains(t, out, "fts/ft.db")
	assert.Contains(t, out, "fts/statuses.csv")
}

// @ft:231
func TestAgentInstructions_MentionsFtStatusCommand(t *testing.T) {
	out := runAgentInstructions(t)

	assert.Contains(t, out, "ft status")
}

// @ft:232
func TestAgentInstructions_GroupsStatusesIntoTiers(t *testing.T) {
	out := strings.ToLower(runAgentInstructions(t))

	assert.Contains(t, out, "system-set")
	assert.Contains(t, out, "human-set")
	assert.Contains(t, out, "agent-set")
	for _, status := range []string{"modified", "removed", "restored", "ready", "accepted", "rejected", "in-progress", "fulfilled"} {
		assert.Contains(t, out, status)
	}
}

// @ft:233
func TestAgentInstructions_StatesAgentMustNotSetHumanOwnedStatuses(t *testing.T) {
	out := runAgentInstructions(t)

	assert.Contains(t, out, "must not set")
}

// @ft:234
func TestAgentInstructions_StatesFulfilledIsWhereAgentStops(t *testing.T) {
	out := runAgentInstructions(t)

	assert.Contains(t, out, "fulfilled")
	assert.Contains(t, out, "Stop there")
}

// @ft:235
func TestAgentInstructions_StatesOnlyReadyScenariosShouldBeImplemented(t *testing.T) {
	out := runAgentInstructions(t)

	assert.Contains(t, out, "Only start")
	assert.Contains(t, out, "ready")
}

// @ft:236
func TestAgentInstructions_DescribesImplementationLoop(t *testing.T) {
	out := runAgentInstructions(t)

	assert.Contains(t, out, "in-progress")
	assert.Contains(t, out, "fulfilled")
	assert.Contains(t, out, "Stop there")
}
