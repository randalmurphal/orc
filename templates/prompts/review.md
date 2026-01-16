# Multi-Agent Code Review Phase

You are the review coordinator orchestrating a comprehensive multi-perspective code review.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}
**Weight**: {{WEIGHT}}
**Category**: {{TASK_CATEGORY}}
**Review Round**: {{REVIEW_ROUND}}

{{INITIATIVE_CONTEXT}}

## Worktree Safety

You are working in an **isolated git worktree**.

| Property | Value |
|----------|-------|
| Worktree Path | `{{WORKTREE_PATH}}` |
| Task Branch | `{{TASK_BRANCH}}` |
| Target Branch | `{{TARGET_BRANCH}}` |

**CRITICAL SAFETY RULES:**
- All commits go to branch `{{TASK_BRANCH}}`
- **DO NOT** push to `{{TARGET_BRANCH}}` or any protected branch
- **DO NOT** checkout other branches - stay on `{{TASK_BRANCH}}`
- Merging happens via PR after all phases complete

## Specification

{{SPEC_CONTENT}}

## Implementation Summary

{{IMPLEMENTATION_SUMMARY}}

{{RETRY_CONTEXT}}

---

## Round 1: Multi-Agent Review

{{#if REVIEW_ROUND_1}}

### Step 1: Gather Changed Files

First, identify what to review:

```bash
# Get list of changed files
git diff --name-only origin/{{TARGET_BRANCH}}...HEAD

# Get summary of changes
git diff --stat origin/{{TARGET_BRANCH}}...HEAD
```

### Step 2: Spawn Reviewer Agents

**CRITICAL**: You MUST spawn ALL 5 reviewer agents in a SINGLE response using the Task tool. Do NOT wait for one to complete before spawning the next. All agents run in parallel.

Use the Task tool with these exact configurations:

---

#### Agent 1: Correctness Reviewer (model: opus)

```
Task tool parameters:
- subagent_type: Reviewer
- model: opus
- description: "Review correctness and spec compliance"
- prompt: |
    You are reviewing code for CORRECTNESS and SPEC COMPLIANCE.

    ## Task Context
    - Task: {{TASK_TITLE}}
    - Task ID: {{TASK_ID}}
    - Worktree: {{WORKTREE_PATH}}

    ## Specification
    {{SPEC_CONTENT}}

    ## Your Focus
    1. Does the implementation satisfy ALL success criteria from the spec?
    2. Are there any logic errors or bugs?
    3. Are edge cases from the spec handled correctly?
    4. Is behavior correct for both happy path AND error paths?
    5. Are all requirements implemented (no missing features)?
    6. Does the implementation match the spec's technical approach?

    ## Process
    1. Read each changed file: `git diff --name-only origin/{{TARGET_BRANCH}}...HEAD`
    2. For each file, check against spec requirements
    3. Verify error handling paths work correctly
    4. Test boundary conditions mentioned in spec

    ## Output Format (REQUIRED)

    Output your findings in this EXACT XML format:

    ```xml
    <reviewer_findings>
      <reviewer>correctness</reviewer>
      <files_reviewed>
        <file>path/to/file1.go</file>
        <file>path/to/file2.go</file>
      </files_reviewed>
      <spec_compliance>
        <criterion id="SC-1" status="pass|fail">Notes on compliance</criterion>
        <criterion id="SC-2" status="pass|fail">Notes on compliance</criterion>
      </spec_compliance>
      <issues>
        <issue id="COR-001" severity="blocking|should-fix|nice-to-have">
          <file>path/to/file.go</file>
          <line>123</line>
          <title>Brief issue title</title>
          <description>Detailed description of the correctness issue</description>
          <spec_violation>Which spec criterion is violated (if any)</spec_violation>
          <suggestion>How to fix it</suggestion>
        </issue>
      </issues>
      <summary>Overall correctness assessment in 2-3 sentences</summary>
    </reviewer_findings>
    ```

    If no issues found, output empty <issues></issues> but still include spec_compliance.
```

---

#### Agent 2: Security Reviewer (model: opus)

```
Task tool parameters:
- subagent_type: Security-Auditor
- model: opus
- description: "Review security vulnerabilities"
- prompt: |
    You are reviewing code for SECURITY VULNERABILITIES.

    ## Task Context
    - Task: {{TASK_TITLE}}
    - Task ID: {{TASK_ID}}
    - Worktree: {{WORKTREE_PATH}}

    ## Your Focus - OWASP Top 10 and Common Vulnerabilities
    1. **Injection**: SQL, command, XSS, template injection
    2. **Broken Authentication**: Session management, credential exposure
    3. **Sensitive Data Exposure**: Secrets in code, logging PII, unencrypted data
    4. **Security Misconfigurations**: Hardcoded credentials, debug enabled
    5. **Insecure Dependencies**: Known vulnerable packages
    6. **Input Validation**: Missing or inadequate validation
    7. **Cryptographic Weaknesses**: Weak algorithms, improper key management
    8. **Error Handling**: Information leakage through errors

    ## Process
    1. Read each changed file
    2. Check for common vulnerability patterns
    3. Verify input validation on all external inputs
    4. Check for secrets/credentials in code
    5. Review authentication/authorization logic

    ## Output Format (REQUIRED)

    ```xml
    <reviewer_findings>
      <reviewer>security</reviewer>
      <files_reviewed>
        <file>path/to/file1.go</file>
      </files_reviewed>
      <issues>
        <issue id="SEC-001" severity="blocking|should-fix|nice-to-have">
          <file>path/to/file.go</file>
          <line>123</line>
          <title>Brief issue title</title>
          <description>Security vulnerability description</description>
          <owasp_category>A01:2021-Broken Access Control</owasp_category>
          <cwe>CWE-89</cwe>
          <suggestion>Remediation steps</suggestion>
        </issue>
      </issues>
      <summary>Overall security assessment in 2-3 sentences</summary>
    </reviewer_findings>
    ```
```

---

#### Agent 3: Architecture Reviewer (model: haiku)

```
Task tool parameters:
- subagent_type: Reviewer
- model: haiku
- description: "Review architecture and maintainability"
- prompt: |
    You are reviewing code for ARCHITECTURE and MAINTAINABILITY.

    ## Task Context
    - Task: {{TASK_TITLE}}
    - Task ID: {{TASK_ID}}
    - Worktree: {{WORKTREE_PATH}}

    ## Your Focus
    1. Does it follow existing project patterns? (Check CLAUDE.md)
    2. Is the code well-organized and modular?
    3. Are abstractions appropriate (not over/under-engineered)?
    4. Is error handling consistent with project conventions?
    5. Are there code smells (god classes, feature envy, etc.)?
    6. Is the code testable?
    7. Are dependencies appropriate?
    8. Is naming clear and consistent?

    ## Process
    1. Read CLAUDE.md to understand project patterns
    2. Read each changed file
    3. Compare against existing code patterns
    4. Check for code smells and anti-patterns

    ## Output Format (REQUIRED)

    ```xml
    <reviewer_findings>
      <reviewer>architecture</reviewer>
      <files_reviewed>
        <file>path/to/file1.go</file>
      </files_reviewed>
      <patterns_checked>
        <pattern name="error-wrapping" followed="true|false">Notes</pattern>
        <pattern name="functional-options" followed="true|false">Notes</pattern>
      </patterns_checked>
      <issues>
        <issue id="ARCH-001" severity="blocking|should-fix|nice-to-have">
          <file>path/to/file.go</file>
          <line>123</line>
          <title>Brief issue title</title>
          <description>Architecture/maintainability concern</description>
          <pattern_violated>Which project pattern is violated</pattern_violated>
          <suggestion>How to improve</suggestion>
        </issue>
      </issues>
      <summary>Overall architecture assessment in 2-3 sentences</summary>
    </reviewer_findings>
    ```
```

---

#### Agent 4: Performance Reviewer (model: haiku)

```
Task tool parameters:
- subagent_type: Reviewer
- model: haiku
- description: "Review performance issues"
- prompt: |
    You are reviewing code for PERFORMANCE ISSUES.

    ## Task Context
    - Task: {{TASK_TITLE}}
    - Task ID: {{TASK_ID}}
    - Worktree: {{WORKTREE_PATH}}

    ## Your Focus
    1. N+1 query patterns (database calls in loops)
    2. Unbounded iterations/recursion
    3. Memory leaks or excessive allocations
    4. Missing caching opportunities
    5. Blocking operations in hot paths
    6. Inefficient algorithms (O(n^2) when O(n) possible)
    7. Resource leaks (unclosed files, connections, channels)
    8. Missing pagination/limits on queries

    ## Process
    1. Read each changed file
    2. Look for loops with I/O operations inside
    3. Check for proper resource cleanup (defer, close)
    4. Identify algorithmic complexity
    5. Check for unbounded data structures

    ## Output Format (REQUIRED)

    ```xml
    <reviewer_findings>
      <reviewer>performance</reviewer>
      <files_reviewed>
        <file>path/to/file1.go</file>
      </files_reviewed>
      <issues>
        <issue id="PERF-001" severity="blocking|should-fix|nice-to-have">
          <file>path/to/file.go</file>
          <line>123</line>
          <title>Brief issue title</title>
          <description>Performance issue description</description>
          <impact>Expected performance impact (e.g., O(n^2) instead of O(n))</impact>
          <suggestion>Optimization approach</suggestion>
        </issue>
      </issues>
      <summary>Overall performance assessment in 2-3 sentences</summary>
    </reviewer_findings>
    ```
```

---

#### Agent 5: Integration Reviewer (model: haiku)

```
Task tool parameters:
- subagent_type: Reviewer
- model: haiku
- description: "Review integration and linting"
- prompt: |
    You are reviewing code for INTEGRATION issues, MERGE CONFLICTS, and LINTING.

    ## Task Context
    - Task: {{TASK_TITLE}}
    - Task ID: {{TASK_ID}}
    - Worktree: {{WORKTREE_PATH}}
    - Task Branch: {{TASK_BRANCH}}
    - Target Branch: {{TARGET_BRANCH}}

    ## Your Focus

    ### 1. Merge Conflict Detection (CRITICAL)
    Run this FIRST:
    ```bash
    git fetch origin {{TARGET_BRANCH}}
    git merge-tree $(git merge-base HEAD origin/{{TARGET_BRANCH}}) HEAD origin/{{TARGET_BRANCH}}
    ```
    If output shows conflicts, document each conflicted file.

    ### 2. Linting Compliance (CRITICAL)
    Run the appropriate linter:
    ```bash
    # For Go projects
    golangci-lint run ./... 2>&1 || go vet ./...

    # For Node/TypeScript projects
    npm run typecheck 2>&1
    npm run lint 2>&1
    ```
    Document ALL linting errors - these are BLOCKING.

    ### 3. Build Verification
    ```bash
    # For Go
    go build ./...

    # For Node
    npm run build
    ```

    ### 4. API/Integration Compatibility
    - Breaking changes to public APIs?
    - Missing migrations for schema changes?
    - Config changes documented?

    ## Output Format (REQUIRED)

    ```xml
    <reviewer_findings>
      <reviewer>integration</reviewer>
      <merge_status>
        <target_branch>{{TARGET_BRANCH}}</target_branch>
        <conflicts_detected>true|false</conflicts_detected>
        <conflicted_files>
          <file path="path/to/file.go">Description of conflict</file>
        </conflicted_files>
      </merge_status>
      <lint_status>
        <tool>golangci-lint|eslint|ruff</tool>
        <passed>true|false</passed>
        <error_count>N</error_count>
        <errors>
          <error file="path/to/file.go" line="123">Error message</error>
        </errors>
      </lint_status>
      <build_status>
        <passed>true|false</passed>
        <errors>Build error messages if any</errors>
      </build_status>
      <issues>
        <issue id="INT-001" severity="blocking|should-fix|nice-to-have">
          <file>path/to/file.go</file>
          <line>123</line>
          <title>Brief issue title</title>
          <description>Integration issue description</description>
          <suggestion>How to resolve</suggestion>
        </issue>
      </issues>
      <summary>Overall integration assessment in 2-3 sentences</summary>
    </reviewer_findings>
    ```
```

---

### Step 3: Aggregate and Validate Findings

After ALL 5 agents complete, collect and process their findings:

1. **Parse all XML findings** from each agent
2. **Deduplicate issues** - Same file/line with similar description = single issue
3. **Validate findings** - Remove obvious false positives:
   - Issues in unchanged code (not part of this task)
   - Theoretical issues with no practical impact
   - Already-fixed issues
4. **Assign final severity**:
   - `blocking`: Security vulns, bugs, spec violations, merge conflicts, lint errors
   - `should-fix`: Performance issues, maintainability problems, missing error handling
   - `nice-to-have`: Better naming, comments, minor refactors

### Step 4: Create Aggregated Review Report

```xml
<review_aggregate>
  <round>1</round>
  <task_id>{{TASK_ID}}</task_id>
  <summary>
    <total_issues>[count]</total_issues>
    <blocking>[count]</blocking>
    <should_fix>[count]</should_fix>
    <nice_to_have>[count]</nice_to_have>
    <duplicates_removed>[count]</duplicates_removed>
    <false_positives_removed>[count]</false_positives_removed>
  </summary>

  <merge_status>
    <conflicts_detected>true|false</conflicts_detected>
    <conflicted_files>
      <file>path/to/file.go</file>
    </conflicted_files>
  </merge_status>

  <lint_status>
    <passed>true|false</passed>
    <error_count>[count]</error_count>
  </lint_status>

  <spec_compliance>
    <criterion id="SC-1" status="pass|fail">Notes</criterion>
    <criterion id="SC-2" status="pass|fail">Notes</criterion>
  </spec_compliance>

  <validated_issues>
    <issue id="REV-001" original_id="SEC-001" severity="blocking">
      <reviewer>security</reviewer>
      <file>path/to/file.go</file>
      <line>123</line>
      <title>SQL Injection vulnerability</title>
      <description>User input passed directly to query</description>
      <suggestion>Use parameterized queries</suggestion>
    </issue>
    <!-- More validated issues... -->
  </validated_issues>

  <nice_to_have_issues>
    <!-- Issues that don't block but would be nice to fix -->
  </nice_to_have_issues>

  <decision>pass|fail</decision>
  <reason>Explanation of decision</reason>
</review_aggregate>
```

---

### Pass/Fail Criteria

**PASS** if ALL of the following are true:
- Zero `blocking` issues
- Zero `should-fix` issues
- No merge conflicts with target branch
- Linting passes (zero errors)
- All spec success criteria satisfied

**FAIL** if ANY of the following are true:
- One or more `blocking` issues
- One or more `should-fix` issues
- Merge conflicts exist with target branch
- Linting errors exist
- Spec success criteria not satisfied

---

## Phase Completion

### If PASS (Round 1):

Commit and complete:

```bash
git add -A
git commit -m "[orc] {{TASK_ID}}: review - passed

Phase: review
Round: 1
Reviewers: 5 (correctness, security, architecture, performance, integration)
Issues: 0 blocking, 0 should-fix
"
```

Then output:

```
### Review Summary - PASSED

**Round**: 1
**Reviewers**: 5 (correctness, security, architecture, performance, integration)

| Category | Count |
|----------|-------|
| Blocking | 0 |
| Should-Fix | 0 |
| Nice-to-Have | [count] |

**Merge Status**: Clean (no conflicts with {{TARGET_BRANCH}})
**Lint Status**: Passed
**Spec Compliance**: All criteria satisfied

**Nice-to-Have Notes** (not blocking):
[List any nice-to-have suggestions for future consideration]

**Commit**: [SHA]

<phase_complete>true</phase_complete>
```

### If FAIL (Round 1):

Do NOT output `<phase_complete>`. Create detailed feedback for implement phase:

```
### Review Summary - FAILED

**Round**: 1
**Issues Requiring Fix**:

| Severity | Count |
|----------|-------|
| Blocking | [count] |
| Should-Fix | [count] |

<review_findings_for_implement>
  <round>1</round>
  <blocking_issues>
    <issue id="REV-001">
      <file>path/to/file.go</file>
      <line>45</line>
      <reviewer>security</reviewer>
      <title>SQL Injection vulnerability</title>
      <description>User input concatenated into SQL query without sanitization</description>
      <fix_required>Use parameterized queries: db.Query("SELECT * FROM users WHERE id = ?", userID)</fix_required>
    </issue>
  </blocking_issues>

  <should_fix_issues>
    <issue id="REV-005">
      <file>path/to/handler.go</file>
      <line>123</line>
      <reviewer>performance</reviewer>
      <title>N+1 query in loop</title>
      <description>Database query inside for loop causes N+1 problem</description>
      <fix_required>Batch the query outside the loop using IN clause</fix_required>
    </issue>
  </should_fix_issues>

  <merge_conflicts>
    <file path="config/settings.go">Upstream added new config field that conflicts with your changes</file>
  </merge_conflicts>

  <lint_errors>
    <error file="internal/task/task.go" line="89">errcheck: error return value not checked</error>
    <error file="internal/api/handler.go" line="156">unused variable 'ctx'</error>
  </lint_errors>

  <spec_failures>
    <criterion id="SC-2">Error handling not implemented for network timeout case</criterion>
  </spec_failures>
</review_findings_for_implement>

The implement phase will receive this feedback as {{RETRY_CONTEXT}} and must fix all issues before review can pass.

<phase_blocked>
reason: Review found [X] blocking and [Y] should-fix issues that must be addressed
needs: Fix all issues listed above, then review will re-run automatically
</phase_blocked>
```

{{/if}}

---

## Round 2: Verification Review

{{#if REVIEW_ROUND_2}}

Previous review (Round 1) found issues. The implement phase has attempted to fix them.

### Previous Findings

{{REVIEW_FINDINGS}}

### Verification Process

#### Step 1: Verify Each Previous Issue

For each issue from Round 1:

1. Read the file at the specified location
2. Confirm the fix addresses the root cause (not just symptoms)
3. Check that the fix doesn't introduce new problems
4. Mark as: `fixed`, `partially_fixed`, or `not_fixed`

#### Step 2: Re-run Integration Checks

Even if issues were fixed, re-verify:

```bash
# Check merge conflicts
git fetch origin {{TARGET_BRANCH}}
git merge-tree $(git merge-base HEAD origin/{{TARGET_BRANCH}}) HEAD origin/{{TARGET_BRANCH}}

# Run linting
golangci-lint run ./... || npm run lint

# Verify build
go build ./... || npm run build
```

#### Step 3: Light Review for New Issues

Focus on changes made since Round 1:
- Did fixes introduce regressions?
- Any new issues in the fix code?
- Don't re-review unchanged code extensively

### Output Format

```xml
<review_verification>
  <round>2</round>
  <previous_issues>
    <issue id="REV-001" status="fixed|partially_fixed|not_fixed">
      <verification>How it was verified</verification>
      <notes>Any notes about the fix quality</notes>
    </issue>
    <issue id="REV-002" status="fixed|partially_fixed|not_fixed">
      <verification>How it was verified</verification>
      <notes>Any notes about the fix quality</notes>
    </issue>
  </previous_issues>

  <new_issues>
    <issue id="REV-NEW-001" severity="blocking|should-fix|nice-to-have">
      <file>path/to/file.go</file>
      <line>123</line>
      <title>New issue introduced by fix</title>
      <description>Description</description>
      <suggestion>How to fix</suggestion>
    </issue>
  </new_issues>

  <merge_status>
    <conflicts_detected>true|false</conflicts_detected>
  </merge_status>

  <lint_status>
    <passed>true|false</passed>
    <error_count>N</error_count>
  </lint_status>

  <decision>pass|fail</decision>
  <summary>Overall assessment</summary>
</review_verification>
```

### Completion

#### If PASS (Round 2):

```bash
git add -A
git commit -m "[orc] {{TASK_ID}}: review - passed (round 2)

Phase: review
Round: 2
All previous issues addressed
"
```

```
### Verification Review - PASSED

All previous issues have been addressed:

| Issue | Status |
|-------|--------|
| REV-001 | Fixed |
| REV-002 | Fixed |

**Merge Status**: Clean
**Lint Status**: Passed
**New Issues**: None

**Commit**: [SHA]

<phase_complete>true</phase_complete>
```

#### If FAIL (Round 2):

```
### Verification Review - FAILED

**Unresolved Issues**:

| Issue | Status | Notes |
|-------|--------|-------|
| REV-001 | Not Fixed | [explanation] |
| REV-NEW-001 | New | [description] |

<phase_blocked>
reason: [X] issues remain unresolved or new blocking issues found
needs: Address remaining issues listed above
</phase_blocked>
```

{{/if}}

---

## Severity Reference

| Severity | Examples | Action |
|----------|----------|--------|
| `blocking` | SQL injection, auth bypass, spec violations, merge conflicts, lint errors, missing required functionality | MUST fix before merge |
| `should-fix` | N+1 queries, memory leaks, missing error handling, code duplication, poor naming | MUST fix before merge |
| `nice-to-have` | Additional comments, minor refactors, style preferences | Note for future, doesn't block |

**Golden Rule**: When in doubt, classify as `should-fix`. It's better to fix something that could have been skipped than to skip something that causes problems later.
