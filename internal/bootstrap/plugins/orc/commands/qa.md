---
description: "Run QA session (e2e tests, documentation)"
argument-hint: "[TASK-ID]"
---

# Orc QA Command

Run a QA session for the specified task after review passes.

## QA Process

As a QA engineer, validate the implementation:

### 1. E2E Tests

Write and run end-to-end tests:
- Cover the happy path
- Cover edge cases
- Test error scenarios
- Ensure integration works

```bash
# Run existing tests first
orc diff TASK-XXX  # See what changed
# Then run appropriate test command based on project
```

### 2. Regression Validation

Ensure existing functionality still works:
- Run the full test suite
- Check for any regressions introduced
- Verify backwards compatibility if applicable

### 3. Documentation

Generate or update documentation:
- User-facing feature docs
- API documentation if applicable
- Update README if needed
- Add inline code comments for complex logic

### 4. Manual Testing Scripts

Create scripts for manual validation:
- Quick smoke test commands
- Example usage demonstrations
- Testing checklist

## Configuration

Check `.orc/config.yaml` for QA settings:
```yaml
qa:
  enabled: true
  skip_for_weights:
    - trivial
  require_e2e: false
  generate_docs: true
```

If task weight is in `skip_for_weights`, QA may be skipped.

## Completion

When QA is complete and all tests pass:
```xml
<qa_complete>true</qa_complete>
```

If QA finds issues that need fixing:
```xml
<qa_blocked>
  <reason>Tests failing</reason>
  <failures>
    <failure>test_user_auth: assertion failed</failure>
  </failures>
</qa_blocked>
```

## Output Summary

At the end, provide a QA summary:

```markdown
## QA Summary for TASK-XXX

### Tests
- Unit tests: PASS (23/23)
- Integration tests: PASS (5/5)
- E2E tests: PASS (3/3)

### Documentation
- Updated: README.md, docs/feature.md
- Generated: API docs

### Manual Testing
- Created: scripts/test-feature.sh

### Status: COMPLETE
```
