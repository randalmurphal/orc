---
description: "Start multi-round code review for task"
argument-hint: "[TASK-ID]"
---

# Orc Review Command

Start a multi-round code review session for the specified task.

## Review Process

The review runs in two rounds:

### Round 1: Exploratory Review

As a senior engineer, examine the implemented code:

1. **Read the implementation**
   - Check all modified files
   - Understand the changes made
   - Compare against the spec/requirements

2. **Identify gaps and issues**
   - Architecture alignment
   - Edge cases not covered
   - Integration concerns
   - Security considerations
   - Performance implications

3. **Output findings**
   ```xml
   <review_findings>
     <round>1</round>
     <issues>
       <issue severity="high">Description of high-severity issue</issue>
       <issue severity="medium">Description of medium issue</issue>
       <issue severity="low">Description of low-priority issue</issue>
     </issues>
     <questions>
       <question>Clarifying question about the implementation</question>
     </questions>
   </review_findings>
   ```

### Round 2: Validation Review

After implementation agent addresses feedback:

1. **Re-read the code with fresh perspective**
2. **Verify all issues addressed**
3. **Check for regressions**
4. **Make final decision**

Output decision:
```xml
<review_decision>
  <status>pass|fail|needs_user_input</status>
  <gaps_addressed>true|false</gaps_addressed>
  <remaining_issues>
    <issue>Any remaining issue</issue>
  </remaining_issues>
  <user_questions>
    <question>Architecture question needing user decision</question>
  </user_questions>
</review_decision>
```

## User Input Required

If the review identifies questions requiring user input (architecture decisions, vision clarification):

1. Output with `<status>needs_user_input</status>`
2. List questions in `<user_questions>`
3. The orchestrator will surface these to the user
4. Review resumes when answers provided

## Configuration

Check `.orc/config.yaml` for review settings:
```yaml
review:
  enabled: true
  rounds: 2
  require_pass: true
```

## On Failure

If review fails:
- Task returns to implement phase
- Review context passed to implementation
- Implementation agent fixes issues
- Review runs again
