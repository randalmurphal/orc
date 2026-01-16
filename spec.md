# Specification: Create Textarea primitive component

## Problem Statement

The codebase has inconsistent textarea styling across components (TaskEditModal, CommentsTab, InlineCommentThread, InitiativeDetail). Each uses slightly different backgrounds (`--bg-tertiary` vs `--bg-secondary`), focus styles (some with ring, some without), and lacks shared features like auto-resize and character count. A unified Textarea primitive will provide consistent styling and behavior.

## Success Criteria

- [ ] `Textarea.tsx` exists at `web/src/components/ui/Textarea.tsx`
- [ ] `Textarea.css` exists at `web/src/components/ui/Textarea.css`
- [ ] Component exported from `web/src/components/ui/index.ts`
- [ ] Default styling matches spec: `--bg-secondary` background, `--border-default` border, `--radius-md` radius
- [ ] Focus state shows accent border with glow ring (`0 0 0 2px var(--accent-glow)`)
- [ ] Hover state shows `--border-strong` border
- [ ] Error variant shows `--status-danger` border
- [ ] Disabled state shows `opacity: 0.5` and `cursor: not-allowed`
- [ ] Auto-resize grows textarea height up to `maxHeight` (default 300px)
- [ ] Auto-resize triggers scrollbar when content exceeds `maxHeight`
- [ ] Character count displays below textarea when `showCount={true}` and `maxLength` set
- [ ] Character count shows warning color at >= 90% capacity
- [ ] Error message displays below textarea when `error` prop provided
- [ ] Component uses `forwardRef` for ref forwarding
- [ ] Accessibility: `aria-invalid` set when error state
- [ ] Accessibility: `aria-describedby` links error message and character count

## Testing Requirements

- [ ] Unit test: Renders with default props
- [ ] Unit test: Calls onChange when text entered
- [ ] Unit test: Respects disabled state (does not call onChange)
- [ ] Unit test: Auto-resize increases textarea height with content
- [ ] Unit test: Auto-resize respects maxHeight limit
- [ ] Unit test: Character count displays "X/Y" format
- [ ] Unit test: Character count shows warning class at >= 90% capacity
- [ ] Unit test: Error state displays error message
- [ ] Unit test: Error state applies error class to textarea
- [ ] Visual snapshot: Default state
- [ ] Visual snapshot: Focus state
- [ ] Visual snapshot: Error state with message
- [ ] Visual snapshot: With character count (normal)
- [ ] Visual snapshot: With character count (warning)
- [ ] Visual snapshot: Auto-resize expanded

## Scope

### In Scope

- Textarea primitive component with consistent styling
- Auto-resize functionality via scrollHeight calculation
- Character count display with warning state
- Error state styling and message display
- Unit tests in `Textarea.test.tsx`
- Visual snapshot tests
- CSS following Button.css patterns (CSS variables, state variants)

### Out of Scope

- Migration of existing textareas to use new component (separate task)
- Markdown preview or rich text support
- File attachment or drag-drop support
- Emoji picker integration

## Technical Approach

### Component Architecture

Follow Button.tsx patterns:
- `forwardRef` for ref forwarding
- Props interface extending `React.TextareaHTMLAttributes<HTMLTextAreaElement>`
- Class composition via array filter/join
- Separate CSS file with design token usage

### Auto-Resize Implementation

```typescript
const adjustHeight = useCallback(() => {
  if (!textareaRef.current || !autoResize) return;
  textareaRef.current.style.height = 'auto';
  const scrollHeight = textareaRef.current.scrollHeight;
  textareaRef.current.style.height = `${Math.min(scrollHeight, maxHeight)}px`;
}, [autoResize, maxHeight]);

useEffect(() => {
  adjustHeight();
}, [value, adjustHeight]);
```

### Character Count Implementation

- Use internal `useId()` for unique description IDs
- Combine IDs for `aria-describedby`: error message + character count
- Calculate percentage: `(value.length / maxLength) * 100`
- Apply warning class when percentage >= 90

### Files to Modify

| File | Change |
|------|--------|
| `web/src/components/ui/Textarea.tsx` | Create component |
| `web/src/components/ui/Textarea.css` | Create styles |
| `web/src/components/ui/index.ts` | Add export |
| `web/src/components/ui/Textarea.test.tsx` | Create unit tests |
| `web/e2e/visual.spec.ts` | Add visual snapshots |

## Feature Details

### User Story

As a developer, I want a consistent Textarea component so that all text input areas have uniform styling, behavior, and accessibility features without duplicating CSS.

### Acceptance Criteria

1. Textarea renders with default 3-row height (min-height: 80px)
2. When `autoResize` is true, height grows with content
3. When content exceeds `maxHeight`, scrollbar appears
4. When `showCount` and `maxLength` set, character count shows below
5. When characters >= 90% of max, count turns red
6. When `error` prop set, border turns red and message appears below
7. Component is keyboard accessible (Tab focus, standard textarea interactions)
8. Screen readers announce character count and error messages

### Props Interface

```typescript
interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  /** Visual variant - default or error state */
  variant?: 'default' | 'error';
  /** Enable auto-resize based on content */
  autoResize?: boolean;
  /** Maximum height in pixels when auto-resize enabled (default: 300) */
  maxHeight?: number;
  /** Show character count when maxLength is set */
  showCount?: boolean;
  /** Error message to display below textarea */
  error?: string;
}
```

### CSS Class Structure

```css
.textarea                  /* Base styles */
.textarea--error           /* Error variant border */
.textarea--disabled        /* Disabled opacity */
.textarea-wrapper          /* Container for textarea + count + error */
.textarea-count            /* Character count */
.textarea-count--warning   /* Warning state (>=90%) */
.textarea-error            /* Error message text */
```

## Design Tokens Used

| Token | Usage |
|-------|-------|
| `--font-body` | Font family |
| `--text-sm` | Font size |
| `--bg-secondary` | Background |
| `--border-default` | Default border |
| `--border-strong` | Hover border |
| `--accent-primary` | Focus border |
| `--accent-glow` | Focus ring |
| `--status-danger` | Error border/text |
| `--text-primary` | Input text |
| `--text-muted` | Placeholder, count |
| `--text-xs` | Count, error font size |
| `--radius-md` | Border radius |
| `--space-2`, `--space-3` | Padding |
| `--duration-fast` | Transition duration |
| `--ease-out` | Transition easing |
