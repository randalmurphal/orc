# Planning Session: {{.Title}}

{{if .TaskID}}
## Task: {{.TaskID}}
{{if .Weight}}- **Weight**: {{.Weight}}{{end}}
{{if .TaskDescription}}- **Description**: {{.TaskDescription}}{{end}}
{{end}}

{{if .HasInitiative}}
## Initiative: {{.InitiativeID}} - {{.InitiativeTitle}}
{{if .InitiativeVision}}**Vision**: {{.InitiativeVision}}{{end}}
{{if .InitiativeDecisions}}
**Decisions**:
{{.InitiativeDecisions}}{{end}}
{{end}}

## Your Role

Create a specification for **{{.Title}}** that will guide implementation.

### How to Work

1. **Read CLAUDE.md first** - The project's CLAUDE.md contains coding standards, patterns, and context. Use it.
2. **Research the codebase** - Look at existing patterns before proposing anything
3. **Ask questions** - Don't assume. Clarify requirements with the user.
4. **Propose and refine** - Suggest an approach, iterate until clear
5. **Create the spec** - Write a focused document

{{if eq .Weight "trivial"}}
### Spec Requirements (Trivial)

Brief spec with:
- **Intent**: What and why (1-2 sentences each)
- **Success Criteria**: 1-2 testable items
{{else}}
### Spec Requirements

Required sections:

**## Intent**
- **What**: 1-2 sentences describing the change
- **Why**: 1-2 sentences explaining the motivation

**## Success Criteria**
- Specific, testable outcomes
- Use checkbox format: `- [ ] Criterion`

**## Testing**
- What tests are needed (unit, integration, e2e)
- Manual verification steps

Optional sections (include if relevant):
- **Scope**: What's in/out of scope
- **Technical Approach**: Implementation strategy, key decisions
{{end}}

{{if .TaskID}}
### Save Location
Save to: `.orc/tasks/{{.TaskID}}/spec.md`
{{else}}
### Save Location
Save to: `.orc/specs/<feature-name>.md` (lowercase with hyphens)
{{end}}

{{if .CreateTasks}}
### Task Generation
After the spec is approved, create tasks:
```
orc new "<title>" --weight <trivial|small|medium|large>
```
{{end}}

### Completion

When the spec is complete and saved:
```
<spec_complete>true</spec_complete>
```

---

Start by asking the user about their requirements for **{{.Title}}**.
