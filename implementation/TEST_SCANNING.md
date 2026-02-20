# `ft` — Test Link Scanning

Scanning the project directory for `@ft:<id>` tags in non-`.ft` files to discover test links.

---

## Scope

`ft sync` and the daemon scan test files in the project directory (the directory where `ft init` was run) for `@ft:<id>` patterns.

### Test File Patterns

Only files matching hardcoded test file patterns are scanned:

- `*_test.go`

More patterns can be added later as needed.

### Excluded

- The `.git/` directory

## Scanning Strategy

### Full Scan (ft sync, daemon startup)

1. Walk the directory tree, respecting gitignore rules
2. For each non-ignored, non-binary file:
   - Read the file and search for `@ft:<id>` patterns
   - Pattern: `@ft:\d+`
   - Record each match: file path, line number, scenario ID
3. Diff the scan results against existing `test_links` in the DB:
   - **New link** — insert a `test_links` row
   - **Existing link** — update `updated_at`
   - **Missing link** (in DB but not in scan) — delete the `test_links` row

### Incremental Scan (daemon file change event)

1. A single file changed — re-scan that file only
2. Extract all `@ft:<id>` patterns from the file
3. Diff against existing `test_links` for that file path
4. Insert/update/delete as needed

## Performance Considerations

### Large Repositories

Scanning every file can be slow in large repos. Mitigations:

- **Pattern matching skips most files** — only `*_test.go` files are scanned, the vast majority of the tree is never read
- **Parallel file reading** — use a worker pool (bounded goroutines) to scan multiple files concurrently

### False Positives

Because scanning is limited to files matching test file patterns, false positives are unlikely. An `@ft:` tag in a test file is a deliberate link.

## Pattern Matching

The regex pattern for scanning:

```
@ft:(\d+)
```

- Captures the numeric ID
- A single line can contain multiple `@ft:` tags (linking one test to multiple scenarios)
- The tag can appear anywhere in the line (in a comment, annotation, string, etc.)
