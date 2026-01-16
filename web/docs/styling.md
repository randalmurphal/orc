# Styling Architecture

The frontend uses a comprehensive design token system with "Mission Control" theme.

## File Structure

| File | Purpose |
|------|---------|
| `src/styles/tokens.css` | Design tokens (colors, typography, spacing) |
| `src/styles/animations.css` | Keyframe animations and utilities |
| `src/index.css` | Global styles, base resets, imports tokens |

**Import order in `index.css`:**
```css
@import './styles/tokens.css';
@import './styles/animations.css';
```

## Color System

| Category | Variables | Description |
|----------|-----------|-------------|
| Backgrounds | `--bg-void` through `--bg-elevated` | 6-level depth scale (#030508 to #2a3a50) |
| Accent | `--accent-primary`, `-secondary`, `-glow`, `-hover` | Electric violet (#a78bfa) + variations |
| Status | `--status-success/warning/danger/info/running` | Semantic colors with `-glow` and `-bg` variants |
| Weight | `--weight-trivial/small/medium/large/greenfield` | Task weight badge colors |
| Text | `--text-primary/secondary/muted/disabled/inverse/accent` | Text hierarchy |
| Border | `--border-subtle/default/strong/focus/glow` | Border colors |

**React-specific aliases:**
```css
--bg-hover: var(--bg-surface);
--accent-primary-hover: var(--accent-hover);
--accent-primary-transparent: var(--accent-subtle);
--status-error: var(--status-danger);
```

## Typography

| Token | Value | Usage |
|-------|-------|-------|
| `--font-display` | Inter | Headings |
| `--font-body` | Inter | Body text |
| `--font-mono` | JetBrains Mono | Code blocks |
| `--text-xs` to `--text-3xl` | 11px to 40px | Font size scale |
| `--font-regular` to `--font-bold` | 400-700 | Font weights |

Font faces: Inter (400/500/600/700) and JetBrains Mono (400/500/600) via @fontsource packages.

## Spacing & Layout

| Token | Value | Usage |
|-------|-------|-------|
| `--space-0` to `--space-32` | 0 to 8rem | Spacing scale (21 values) |
| `--radius-sm` to `--radius-full` | 4px to 9999px | Border radius scale |
| `--sidebar-width-collapsed/expanded` | 60px/220px | Sidebar dimensions |
| `--header-height` | 56px | Header height |

## Effects & Animation

| Category | Tokens |
|----------|--------|
| Shadows | `--shadow-xs` to `--shadow-2xl` (6 levels) |
| Glows | `--shadow-glow-sm/glow/glow-lg`, status-specific glows |
| Durations | `--duration-instant` (50ms) to `--duration-slowest` (700ms) |
| Easings | `--ease-linear/in/out/in-out/bounce/spring` |
| Z-index | `--z-base` (0) to `--z-max` (9999), 11 named layers |

## WCAG Compliance

All colors meet WCAG AA contrast requirements (4.5:1 on dark backgrounds):
- Text colors lightened for contrast on `--bg-secondary`
- Status colors adjusted for accessibility
- Accent color uses #a78bfa (not darker purple) for readability

## Usage Example

```css
.task-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  padding: var(--space-4);
  transition: background var(--duration-fast) var(--ease-out);
}

.task-card:hover {
  background: var(--bg-hover);
}
```
