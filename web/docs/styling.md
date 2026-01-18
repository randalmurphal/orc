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
| Primary | `--primary`, `--primary-bright`, `--primary-dim`, `--primary-glow`, `--primary-border`, `--primary-accessible` | Purple accent (#a855f7) + variations |
| Semantic | `--cyan`, `--orange`, `--green`, `--red`, `--amber`, `--blue` | Status colors with `-dim` variants |
| Text | `--text-primary`, `--text-secondary`, `--text-muted` | Text hierarchy |
| Border | `--border`, `--border-light`, `--border-hover` | Border colors with opacity |

### Semantic Status Aliases

These aliases map semantic names to design system colors:

| Alias | Maps To | Usage |
|-------|---------|-------|
| `--status-success` | `--green` | Success states |
| `--status-warning` | `--amber` | Warning states |
| `--status-danger`, `--status-error` | `--red` | Error states |
| `--status-info` | `--blue` | Info states |
| `--bg-tertiary` | `--bg-surface` | Tertiary background |
| `--accent-primary` | `--primary` | Primary accent |
| `--accent-glow` | `--primary-glow` | Accent glow effect |
| `--accent-subtle` | `--primary-dim` | Subtle accent background |
| `--accent-secondary` | `--cyan` | Secondary accent |

### Overlay Colors

For backdrops and translucent overlays:

| Token | Value | Usage |
|-------|-------|-------|
| `--overlay-light` | `rgba(0,0,0,0.3)` | Light dimming |
| `--overlay-medium` | `rgba(0,0,0,0.5)` | Standard modal backdrop |
| `--overlay-dark` | `rgba(0,0,0,0.6)` | Dark overlay |
| `--overlay-heavy` | `rgba(0,0,0,0.7)` | Heavy dimming |
| `--overlay-opaque` | `rgba(0,0,0,0.9)` | Near-opaque |
| `--overlay-white-subtle` | `rgba(255,255,255,0.1)` | Subtle white tint |
| `--overlay-white-light` | `rgba(255,255,255,0.2)` | Light white tint |
| `--overlay-white-border` | `rgba(255,255,255,0.3)` | White border highlight |

**Note:** Prefer semantic token names for new code.

## Typography

| Token | Value | Usage |
|-------|-------|-------|
| `--font-display` | Inter | Headings |
| `--font-body` | Inter | Body text |
| `--font-mono` | JetBrains Mono | Code blocks |
| `--text-xs` to `--text-5xl` | 8px to 28px | Font size scale (10 sizes) |
| `--font-regular` to `--font-bold` | 400-700 | Font weights |

### Font Size Scale

| Token | Size | Usage |
|-------|------|-------|
| `--text-xs` | 8px (0.5rem) | Tiny labels |
| `--text-sm` | 9px (0.5625rem) | Small text |
| `--text-base` | 11px (0.6875rem) | Body text (default) |
| `--text-md` | 12px (0.75rem) | Slightly larger body |
| `--text-lg` | 13px (0.8125rem) | Emphasized text |
| `--text-xl` | 14px (0.875rem) | Subheadings |
| `--text-2xl` | 16px (1rem) | Section headings |
| `--text-3xl` | 18px (1.125rem) | Page headings |
| `--text-4xl` | 24px (1.5rem) | Large headings |
| `--text-5xl` | 28px (1.75rem) | Display headings |

**Note:** `--text-2xs` is a legacy alias for `--text-xs`.

Font faces: Inter (400/500/600/700) and JetBrains Mono (400/500/600) via @fontsource packages.

## Spacing & Layout

| Token | Value | Usage |
|-------|-------|-------|
| `--space-0` to `--space-32` | 0 to 8rem | Spacing scale (21 values) |
| `--radius-sm` to `--radius-full` | 4px to 9999px | Border radius scale |
| `--sidebar-width-collapsed/expanded` | 56px/260px | Sidebar dimensions |
| `--header-height` | 48px | Header height |

## Effects & Animation

| Category | Tokens |
|----------|--------|
| Shadows | `--shadow-xs` to `--shadow-2xl` (6 levels) |
| Glows | `--shadow-glow-sm/glow/glow-lg`, status-specific glows |
| Durations | `--duration-instant` (50ms) to `--duration-slowest` (700ms) |
| Easings | `--ease-linear/in/out/in-out/bounce/spring` |
| Z-index | `--z-base` (0) to `--z-max` (9999), 11 named layers |

## Light Theme

The design system includes light theme overrides via `[data-theme="light"]` on `:root`:

```css
/* Apply light theme */
document.documentElement.dataset.theme = 'light';
```

Light theme automatically adjusts:
- Backgrounds: Light grays (#f8fafc to #ffffff)
- Text: Dark slate (#0f172a to #64748b)
- Shadows: Reduced opacity for subtlety
- Primary colors: Darker purple for contrast

## WCAG Compliance

Colors are designed for accessibility on both dark and light backgrounds. The design system uses:
- Opacity-based borders for consistent visual hierarchy
- `--primary-accessible` variant for WCAG AA text contrast
- Semantic `-dim` variants for subtle backgrounds

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
