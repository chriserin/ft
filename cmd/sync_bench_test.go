package cmd

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func generateFtFile(name string, scenarioCount int) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Feature: %s\n", name)
	buf.WriteString("  Background:\n")
	buf.WriteString("    Given the system is running\n\n")
	for i := 1; i <= scenarioCount; i++ {
		fmt.Fprintf(&buf, "  Scenario: %s scenario %d\n", name, i)
		fmt.Fprintf(&buf, "    Given precondition %d\n", i)
		fmt.Fprintf(&buf, "    When action %d is taken\n", i)
		fmt.Fprintf(&buf, "    Then result %d is observed\n\n", i)
	}
	return buf.String()
}

func generateTestFile(pkg string, scenarioIDs []int) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "package %s\n\nimport \"testing\"\n\n", pkg)
	for _, id := range scenarioIDs {
		fmt.Fprintf(&buf, "// @ft:%d\n", id)
		fmt.Fprintf(&buf, "func TestScenario%d(t *testing.T) {\n", id)
		buf.WriteString("	t.Run(\"case\", func(t *testing.T) {})\n")
		buf.WriteString("}\n\n")
	}
	return buf.String()
}

func setupBenchProject(b *testing.B, fileCount, scenariosPerFile, testFileCount int) {
	b.Helper()
	dir := b.TempDir()
	orig, err := os.Getwd()
	require.NoError(b, err)
	require.NoError(b, os.Chdir(dir))
	b.Cleanup(func() { os.Chdir(orig) })

	var buf bytes.Buffer
	require.NoError(b, RunInit(&buf))

	for i := 0; i < fileCount; i++ {
		name := fmt.Sprintf("feature_%d", i)
		content := generateFtFile(name, scenariosPerFile)
		require.NoError(b, os.WriteFile(fmt.Sprintf("fts/%s.ft", name), []byte(content), 0o644))
	}

	// Initial sync to assign tags
	buf.Reset()
	require.NoError(b, RunSync(&buf))

	// Generate test files linking to scenario IDs
	if testFileCount > 0 {
		totalScenarios := fileCount * scenariosPerFile
		scenariosPerTestFile := totalScenarios / testFileCount
		if scenariosPerTestFile < 1 {
			scenariosPerTestFile = 1
		}
		require.NoError(b, os.MkdirAll("pkg", 0o755))
		id := 1
		for i := 0; i < testFileCount; i++ {
			var ids []int
			for j := 0; j < scenariosPerTestFile && id <= totalScenarios; j++ {
				ids = append(ids, id)
				id++
			}
			content := generateTestFile("pkg", ids)
			require.NoError(b, os.WriteFile(fmt.Sprintf("pkg/feature_%d_test.go", i), []byte(content), 0o644))
		}

		// Sync again to register test links
		buf.Reset()
		require.NoError(b, RunSync(&buf))
	}
}

// BenchmarkSync_Incremental_Small: 5 files, 10 scenarios each, no changes
func BenchmarkSync_Incremental_Small(b *testing.B) {
	setupBenchProject(b, 5, 10, 0)
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		require.NoError(b, RunSync(&buf))
	}
}

// BenchmarkSync_Incremental_Medium: 20 files, 20 scenarios each, no changes
func BenchmarkSync_Incremental_Medium(b *testing.B) {
	setupBenchProject(b, 20, 20, 0)
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		require.NoError(b, RunSync(&buf))
	}
}

// BenchmarkSync_Incremental_Large: 50 files, 50 scenarios each, no changes
func BenchmarkSync_Incremental_Large(b *testing.B) {
	setupBenchProject(b, 50, 50, 0)
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		require.NoError(b, RunSync(&buf))
	}
}

// BenchmarkSync_WithTestLinks_Small: 5 ft files + 5 test files
func BenchmarkSync_WithTestLinks_Small(b *testing.B) {
	setupBenchProject(b, 5, 10, 5)
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		require.NoError(b, RunSync(&buf))
	}
}

// BenchmarkSync_WithTestLinks_Large: 20 ft files + 50 test files
func BenchmarkSync_WithTestLinks_Large(b *testing.B) {
	setupBenchProject(b, 20, 20, 50)
	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		require.NoError(b, RunSync(&buf))
	}
}

// BenchmarkSync_FirstSync_Small: initial sync of 5 files, 10 scenarios each
func BenchmarkSync_FirstSync_Small(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		dir := b.TempDir()
		orig, _ := os.Getwd()
		os.Chdir(dir)

		var buf bytes.Buffer
		RunInit(&buf)
		for f := 0; f < 5; f++ {
			content := generateFtFile(fmt.Sprintf("feature_%d", f), 10)
			os.WriteFile(fmt.Sprintf("fts/feature_%d.ft", f), []byte(content), 0o644)
		}

		buf.Reset()
		b.StartTimer()
		RunSync(&buf)
		b.StopTimer()
		os.Chdir(orig)
	}
}

// BenchmarkSync_FirstSync_Large: initial sync of 50 files, 50 scenarios each
func BenchmarkSync_FirstSync_Large(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		dir := b.TempDir()
		orig, _ := os.Getwd()
		os.Chdir(dir)

		var buf bytes.Buffer
		RunInit(&buf)
		for f := 0; f < 50; f++ {
			content := generateFtFile(fmt.Sprintf("feature_%d", f), 50)
			os.WriteFile(fmt.Sprintf("fts/feature_%d.ft", f), []byte(content), 0o644)
		}

		buf.Reset()
		b.StartTimer()
		RunSync(&buf)
		b.StopTimer()
		os.Chdir(orig)
	}
}
