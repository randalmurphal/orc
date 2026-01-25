# Dependency Audit

You are performing an automated dependency audit to check for outdated, deprecated, or vulnerable dependencies.

## Objective

Review all project dependencies, identify issues, and update or replace problematic dependencies as needed.

## Context

**Project Root:** {{PROJECT_ROOT}}

## Process

1. **Inventory Dependencies**
   - List all direct and indirect dependencies
   - Note current versions vs latest versions
   - Identify deprecated or archived packages

2. **Security Check**
   - Run security audit tools (bun audit, go mod audit, etc.)
   - Check for known CVEs in dependencies
   - Review security advisories for critical packages

3. **Compatibility Analysis**
   - Check for breaking changes in major version updates
   - Review changelogs for significant updates
   - Identify dependencies that may cause conflicts

4. **Update Strategy**
   For each dependency requiring attention:
   - Minor/patch updates: Update directly
   - Major updates: Assess breaking changes, update if safe
   - Vulnerable packages: Prioritize immediate update or replacement
   - Deprecated packages: Find and migrate to alternatives

5. **Apply Updates**
   - Update dependency files (package.json, go.mod, etc.)
   - Update lock files
   - Run tests to verify compatibility
   - Document any breaking changes encountered

## Output Format

When complete, output ONLY this JSON:

```json
{"status": "complete", "summary": "Dependency audit complete: [count] updates applied"}
```

If blocked (e.g., critical vulnerability requires human decision), output ONLY this JSON:

```json
{"status": "blocked", "reason": "[description of issue]"}
```

## Guidelines

- Don't update major versions without understanding breaking changes
- Always run tests after updates
- Document any manual interventions required
- Prefer security fixes over feature updates
- Keep updates minimal when possible
