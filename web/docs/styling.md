# Styling Architecture

The frontend uses a design token system based on the `example_ui/board.html` reference design.

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

### Core Tokens (New Design System)

| Category | Variables | Description |
|----------|-----------|-------------|
| Backgrounds | `--bg-base`, `--bg-elevated`, `--bg-surface`, `--bg-card`, `--bg-hover` | 5-level depth scale (#050508 to #1c1c26) |
| Primary | `--primary`, `--primary-bright`, `--primary-dim`, `--primary-glow` | Purple accent (#a855f7) + variations |
| Semantic | `--cyan`, `--orange`, `--green`, `--red`, `--amber`, `--blue` | Status colors with `-dim` variants |
| Text | `--text-primary`, `--text-secondary`, `--text-muted` | Text hierarchy |
| Border | `--border`, `--border-light` | Border colors with opacity |

### Legacy Compatibility Aliases

For backward compatibility, old token names map to new values:

| Old Token | Maps To |
|-----------|---------|
| `--bg-void`, `--bg-primary` | `--bg-base` |
| `--bg-secondary` | `--bg-elevated` |
| `--bg-tertiary` | `--bg-surface` |
| `--accent-primary` | `--primary` |
| `--accent-secondary` | `--primary-bright` |
| `--accent-glow` | `--primary-glow` |
| `--status-success` | `--green` |
| `--status-warning` | `--amber` |
| `--status-danger`, `--status-error` | `--red` |
| `--status-info` | `--blue` |
| `--weight-*` | Semantic colors (`--green`, `--blue`, `--amber`, `--primary`) |

**Note:** Prefer new token names for new code. Legacy aliases exist for existing component compatibility.

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
| `--sidebar-width-collapsed/expanded` | 60px/260px | Sidebar dimensions |
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

Colors are designed for accessibility on dark backgrounds. The design system uses opacity-based borders and dim variants for consistent visual hierarchy.

## Usage Example

```css
/* New design system tokens (preferred) */
.task-card {
  background: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  padding: var(--space-4);
  transition: background var(--duration-fast) var(--ease-out);
}

.task-card:hover {
  background: var(--bg-hover);
}

.status-badge {
  color: var(--green);
  background: var(--green-dim);
}
```
