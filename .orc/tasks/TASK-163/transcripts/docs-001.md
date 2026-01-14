# docs - Iteration 1

## Prompt

Update documentation for:

**Task**: Phase 1: React 19 project scaffolding with Vite

**Description**: ## Purpose
Setup React 19 project structure alongside existing Svelte app for parallel development.

## Implementation

### Project Setup
- Create web-react/ directory (parallel to web/)
- Initialize with Vite + React 19 + TypeScript
- Configure same port (5174) for dual-run testing
- Setup build output to match Svelte structure

### Dependencies
- react@19, react-dom@19
- vite, @vitejs/plugin-react
- typescript, @types/react, @types/react-dom

### File Structure
```
web-react/
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── index.css
│   ├── lib/           # Shared utilities
│   ├── components/    # UI components
│   ├── pages/         # Route pages
│   ├── stores/        # Zustand stores
│   └── hooks/         # Custom hooks
├── vite.config.ts
├── tsconfig.json
└── package.json
```

### Configuration
- Copy and adapt vite.config.ts from Svelte
- Setup API proxy to localhost:8080
- Configure path aliases (@/ for src/)

## Success Criteria
- npm run dev starts React app on :5174
- API proxy works (can fetch /api/health)
- TypeScript compiles without errors

1. Update any relevant documentation files
2. Ensure CLAUDE.md reflects the changes if applicable
3. Add/update code comments where needed
4. Update README if user-facing changes were made

Keep iterating until documentation is complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

Good - three files modified/added as expected. Let me summarize what was done:

## Documentation Updated

**1. Root CLAUDE.md**
- Added `web-react/` to the project structure table with link to its CLAUDE.md
- Updated `web/` description to clarify it's the "current" frontend

**2. Created web-react/CLAUDE.md**
- Full documentation for the React 19 migration project
- Tech stack (React 19, Vite, TypeScript, Vitest)
- Directory structure explanation
- Development commands (npm run dev/test/build)
- Port configuration (5174 to avoid conflict with Svelte on 5173)
- Vite and TypeScript config explanation
- Migration strategy overview (5 phases)
- Component mapping from Svelte to React
- Testing approach (shared E2E tests)
- API integration patterns
- Key differences between Svelte 5 and React 19

**3. Makefile Updates**
- Renamed "Frontend" section to "Frontend (Svelte - current)"
- Added new "Frontend (React - migration)" section with:
  - `react-install` - Install React dependencies
  - `react-dev` - Start React dev server on :5174
  - `react-build` - Build React for production
  - `react-test` - Run React unit tests
  - `dev-react` - Start API + React dev servers together
- Updated .PHONY to include new targets

<phase_complete>true</phase_complete>

---
Tokens: 1020059 input, 5296 output, 56320 cache_creation, 962262 cache_read
Complete: true
Blocked: false
