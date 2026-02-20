# `ft` — Test Associations

Tests are linked to scenarios by placing an `@ft:<id>` comment in the test source code. This reuses the same tag that identifies scenarios in `.ft` files.

## Linking a Test

Add a comment containing `@ft:<id>` anywhere in or above a test:

```python
# @ft:42
def test_user_logs_in():
    ...
```

```javascript
// @ft:42
it('should log the user in', () => {
    ...
});
```

The format is language-agnostic — any comment style works as long as `@ft:<id>` appears in the text.

A single test can reference multiple scenarios, and multiple tests can reference the same scenario.

## Discovery

The daemon watches for changes to files matching hardcoded test file patterns. When a matching file changes, it scans its content for `@ft:<id>` patterns.

Test file patterns:
- `*_test.go`

More patterns can be added later. The `.git/` directory is always skipped.

### Database Schema (conceptual)

```
test_links
  id            INTEGER PRIMARY KEY
  scenario_id   INTEGER REFERENCES scenarios(id)
  file_path     TEXT            -- path to the test file
  line_number   INTEGER         -- line where the @ft tag was found
  updated_at    TIMESTAMP
```

## CLI

```
ft tests <id>                  List tests linked to a scenario by its @ft:<id>
```

## Daemon Behavior

- Watches all non-ignored files for changes
- On file change, scans the file content for `@ft:<id>` patterns
- Discovered links are inserted or updated in `test_links`
- If a previously linked `@ft:<id>` comment is removed from a file, the corresponding `test_links` row is deleted
- On startup, performs a full scan of all non-ignored files to reconcile `test_links`
