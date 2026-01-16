# Coverage Improvement

You are performing automated test coverage improvement to ensure adequate test coverage across the codebase.

## Objective

Analyze current test coverage, identify gaps, and add tests to improve coverage to meet the project threshold.

## Context

**Coverage Threshold:** {{COVERAGE_THRESHOLD}}%

**Recent Files Changed:** {{RECENT_CHANGED_FILES}}

## Process

1. **Assess Current Coverage**
   - Run coverage analysis tools
   - Identify files and functions with low coverage
   - Focus on recently changed code
   - Note any untested critical paths

2. **Prioritize Gaps**
   Priority order:
   1. Critical business logic with no tests
   2. Error handling paths
   3. Edge cases in existing tested code
   4. Recently modified code
   5. Utility functions and helpers

3. **Write Tests**
   For each gap:
   - Create unit tests for isolated functions
   - Add integration tests for component interactions
   - Include edge case tests
   - Test error conditions and failure modes

4. **Verify Coverage**
   - Run updated coverage report
   - Ensure threshold is met
   - Verify tests are meaningful (not just coverage farming)

## Output Format

When complete, output:

```xml
<phase_complete>true</phase_complete>
```

If coverage threshold cannot be met, output:

```xml
<phase_blocked>reason: Coverage at X%, threshold is Y%. [Details on remaining gaps]</phase_blocked>
```

## Guidelines

- Write meaningful tests, not just coverage padding
- Test behavior, not implementation details
- Include both positive and negative test cases
- Follow existing test patterns in the codebase
- Don't modify production code to improve coverage
