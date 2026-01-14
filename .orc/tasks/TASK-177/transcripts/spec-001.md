# spec - Iteration 1

## Prompt

Create a specification for this large task:

**Task**: Phase 2: React - Environment and Config pages

**Description**: ## Purpose
Port all environment and configuration pages.

## Pages to Port

### Preferences (/preferences)
- Global settings tab
- Project settings tab
- Environment variables editor

### Environment Hub (/environment)
- Overview with links to sub-pages

### Claude Code Settings
- /environment/docs - CLAUDE.md editor
- /environment/claude/skills - Skill management
- /environment/claude/hooks - Hook configuration
- /environment/claude/agents - Agent definitions
- /environment/claude/tools - Tool permissions
- /environment/claude/mcp - MCP server config
- /environment/claude/plugins - Plugin management
- /environment/claude/statusline - Statusline config

### Orc Settings
- /environment/orchestrator/automation - Profile, timeouts, worktree settings
- /environment/orchestrator/prompts - Phase prompt overrides
- /environment/orchestrator/scripts - Script registry
- /environment/orchestrator/export - Export configuration
- /environment/knowledge - Knowledge queue

## Common Patterns
- Settings forms with validation
- Save/cancel buttons
- JSON/YAML editors where needed
- Preview functionality

## Success Criteria
- All config pages render and save correctly
- Settings persist after save
- Validation errors display properly

Define:
1. Requirements and scope
2. Technical approach
3. Component breakdown (backend, frontend if applicable)
4. API design (if applicable)
5. Success criteria with explicit checkboxes
6. Testing strategy:
   - Unit tests
   - Integration tests
   - E2E tests (using Playwright MCP tools if frontend exists)

Include clear completion criteria:
- What code must be written
- What tests must pass
- What E2E scenarios must work
- What documentation must exist

Keep iterating until the specification is clear and complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

The specification is comprehensive and covers all requirements. Let me verify that the pages in the task description match what's in the spec:

**Task description pages** vs **Spec coverage**:

| Task Description | Spec Coverage |
|------------------|---------------|
| /preferences | ✅ R1 |
| /environment | ✅ R2 (EnvironmentHub) |
| /environment/docs | ✅ R3 (ClaudeMd) |
| /environment/claude/skills | ✅ R4 |
| /environment/claude/hooks | ✅ R5 |
| /environment/claude/agents | ✅ R6 |
| /environment/claude/tools | ✅ R7 |
| /environment/claude/mcp | ✅ R8 |
| /environment/claude/plugins | ✅ R9 |
| /environment/claude/statusline | ✅ R10 |
| /environment/orchestrator/automation | ✅ R11 |
| /environment/orchestrator/prompts | ✅ R12 |
| /environment/orchestrator/scripts | ✅ R13 |
| /environment/orchestrator/export | ✅ R14 |
| /environment/knowledge | ✅ R15 |

All pages are covered! The specification includes:

1. **15 detailed requirements** (R1-R15) - one for each page
2. **API endpoints** - all existing, documented per requirement
3. **Component architecture** - files to create/update
4. **Technical patterns** - state management, data loading, forms
5. **Route updates** - new route structure with grouped navigation
6. **Success criteria** - functionality and quality checklists
7. **Testing strategy** - unit tests, integration tests, E2E tests with patterns
8. **Implementation order** - 6 phases over ~6 days
9. **Dependencies** - blocked by TASK-170 and TASK-164
10. **Risks & mitigations** - marketplace, validation, edge cases
11. **Completion checklist** - code, tests, documentation

<phase_complete>true</phase_complete>

---
Tokens: 711857 input, 9839 output, 86897 cache_creation, 616196 cache_read
Complete: true
Blocked: false
