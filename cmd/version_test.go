package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// @ft:200
func TestRunVersion(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, RunVersion(&buf))
	assert.Equal(t, Version+"\n", buf.String())
}
