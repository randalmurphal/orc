# Task Planning from Specifications

You are analyzing specification documents to create a task breakdown for implementation.

## Specification Documents

The following spec documents have been provided:

{{.SpecFiles}}

## Content

{{.SpecContent}}

## Project Context

- **Project**: {{.ProjectName}}
- **Path**: {{.ProjectPath}}
{{- if .Language}}
- **Language**: {{.Language}}
{{- end}}
{{- if .Frameworks}}
- **Frameworks**: {{.Frameworks}}
{{- end}}

{{if .InitiativeID}}
## Initiative Context

This plan is for initiative **{{.InitiativeID}}**: {{.InitiativeTitle}}

{{if .InitiativeVision}}
**Vision**: {{.InitiativeVision}}
{{end}}

{{if .InitiativeDecisions}}
**Prior Decisions**:
{{.InitiativeDecisions}}
{{end}}
{{end}}

## Instructions

Analyze the specifications and create a task breakdown:

### Step 1: Understand the Scope
- What is being built?
- What are the key components?
- What are the success criteria?

### Step 2: Identify Tasks
Break down the work into discrete tasks:
- Each task should be independently completable
- Prefer smaller tasks (trivial/small) for clarity
- Group related work but keep tasks focused

### Step 3: Determine Dependencies
- Which tasks must complete before others can start?
- Identify parallel work opportunities
- Minimize blocking dependencies

### Step 4: Classify Weights
Use these definitions:
| Weight | Scope | Duration |
|--------|-------|----------|
| trivial | 1 file, <10 lines | Minutes |
| small | 1 component, <100 lines | <1 hour |
| medium | Multiple files, investigation | Hours |
| large | Cross-cutting, significant | Days |

### Step 5: Output the Plan

First, provide a brief summary of your analysis.

Then output the task breakdown in this exact format:

<task_breakdown>
<task id="1">
<title>Short, action-oriented title</title>
<description>What this task accomplishes. Include:
- Key changes
- Files likely affected
- Success criteria</description>
<weight>trivial|small|medium|large</weight>
<depends_on>comma-separated IDs or empty</depends_on>
</task>
<!-- More tasks... -->
</task_breakdown>

## Guidelines

- **Atomic tasks**: Each task should do one thing well
- **Clear descriptions**: Include enough context for implementation
- **Conservative weights**: When uncertain, round UP
- **Minimal dependencies**: Only specify true blockers
- **Testable outcomes**: Each task should have verifiable results

## Example

<task_breakdown>
<task id="1">
<title>Create User model schema</title>
<description>Define the User model with email, password_hash, created_at fields.
- File: internal/models/user.go
- Success: Model compiles, migrations run</description>
<weight>small</weight>
<depends_on></depends_on>
</task>
<task id="2">
<title>Implement password hashing utility</title>
<description>Add bcrypt-based password hashing in auth package.
- File: internal/auth/password.go
- Success: Hash and verify functions with tests</description>
<weight>small</weight>
<depends_on></depends_on>
</task>
<task id="3">
<title>Create registration endpoint</title>
<description>POST /api/auth/register with validation.
- Uses User model and password hashing
- Success: Endpoint returns 201 on valid registration</description>
<weight>medium</weight>
<depends_on>1,2</depends_on>
</task>
</task_breakdown>
