# Specification: Replace Modal.tsx with Radix Dialog

## Problem Statement
Replace the custom Modal component implementation with Radix Dialog to leverage accessible primitives while preserving the existing CSS styling and component API.

## Success Criteria
- [ ] Modal component uses Radix Dialog internally
- [ ] All existing Modal props (`open`, `onClose`, `size`, `title`, `showClose`, `children`) work identically
- [ ] CSS classes `.modal-backdrop`, `.modal-content`, `.modal-header`, `.modal-title`, `.modal-close`, `.modal-body` are preserved
- [ ] Animations work with Radix `data-state` attributes (open/closed)
- [ ] No changes required in consuming components (TaskEditModal, KeyboardShortcutsHelp, InitiativeDetail)
- [ ] Focus trap cycles within modal (Tab/Shift+Tab)
- [ ] Focus returns to trigger element after close
- [ ] Escape key closes modal
- [ ] Click outside (on overlay) closes modal
- [ ] Body scroll prevented when modal open
- [ ] All existing unit tests pass with minimal modification
- [ ] E2E tests that use modals continue to pass

## Testing Requirements
- [ ] Unit test: Modal opens when `open=true`
- [ ] Unit test: Modal closes when `onClose` called
- [ ] Unit test: Title renders when provided
- [ ] Unit test: Size classes apply correctly (sm, md, lg, xl)
- [ ] Unit test: Children render inside content
- [ ] Unit test: Close button hidden when `showClose=false`
- [ ] Unit test: Escape closes modal
- [ ] Unit test: Backdrop click closes modal
- [ ] Unit test: Content click does not close modal
- [ ] Unit test: Proper accessibility attributes (role, aria-modal, aria-labelledby)
- [ ] Unit test: Portal renders to document.body
- [ ] Unit test: Body overflow hidden when open
- [ ] E2E: `bunx playwright test` - all modal-using tests pass

## Scope

### In Scope
- Replace Modal.tsx implementation with Radix Dialog
- Update Modal.css for `data-state` animation triggers
- Update Modal.test.tsx for any Radix-specific behavior changes
- Export types remain unchanged (`Modal`, `ModalSize`)

### Out of Scope
- Changing Modal's external API
- Modifying consuming components (TaskEditModal, KeyboardShortcutsHelp, InitiativeDetail)
- Adding exit animations (nice-to-have, not required)
- Changing modal styling/appearance

## Technical Approach

### Component Mapping
| Current Custom | Radix Component | Notes |
|----------------|-----------------|-------|
| Portal via `createPortal` | `Dialog.Portal` | Built-in portal handling |
| Backdrop div | `Dialog.Overlay` | Class: `.modal-backdrop` |
| Content div | `Dialog.Content` | Class: `.modal-content` |
| Title h2 | `Dialog.Title` | Class: `.modal-title` |
| Close button | `Dialog.Close` | Class: `.modal-close` |
| Custom focus trap | Built-in | Radix handles automatically |
| Custom escape handler | Built-in | Radix handles automatically |
| Custom scroll lock | Built-in | Radix handles automatically |

### Files to Modify
- `web/src/components/overlays/Modal.tsx`: Replace implementation with Radix Dialog
- `web/src/components/overlays/Modal.css`: Add `data-state` selectors for animations
- `web/src/components/overlays/Modal.test.tsx`: Update tests for Radix behavior differences

### Implementation Structure
```tsx
import * as Dialog from '@radix-ui/react-dialog';
import { Icon } from '@/components/ui/Icon';
import './Modal.css';

export type ModalSize = 'sm' | 'md' | 'lg' | 'xl';

interface ModalProps {
  open: boolean;
  onClose: () => void;
  size?: ModalSize;
  title?: string;
  showClose?: boolean;
  children: ReactNode;
}

const sizeClasses: Record<ModalSize, string> = {
  sm: 'max-width-sm',
  md: 'max-width-md',
  lg: 'max-width-lg',
  xl: 'max-width-xl',
};

export function Modal({
  open,
  onClose,
  size = 'md',
  title,
  showClose = true,
  children,
}: ModalProps) {
  return (
    <Dialog.Root open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="modal-backdrop" />
        <Dialog.Content className={`modal-content ${sizeClasses[size]}`}>
          {(title || showClose) && (
            <div className="modal-header">
              {title && <Dialog.Title className="modal-title">{title}</Dialog.Title>}
              {showClose && (
                <Dialog.Close className="modal-close" aria-label="Close modal" title="Close (Esc)">
                  <Icon name="close" size={18} />
                </Dialog.Close>
              )}
            </div>
          )}
          <div className="modal-body">{children}</div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
```

### CSS Changes Required
```css
/* Update animation triggers to use data-state */
.modal-backdrop[data-state='open'] {
  animation: fade-in var(--duration-normal) var(--ease-out);
}

.modal-content[data-state='open'] {
  animation: modal-content-in var(--duration-normal) var(--ease-out);
}
```

### Test Changes Expected
- Radix Dialog.Content has `role="dialog"` automatically
- `aria-labelledby` uses auto-generated IDs - update tests to check relationship exists, not specific ID
- Focus behavior is managed by Radix - focus trap tests may need timing adjustments
- Backdrop element is `Dialog.Overlay`, separate from `Dialog.Content`

## Refactor Analysis

### Before Pattern
- Custom implementation using `createPortal`, `useEffect`, `useRef`
- Manual focus trap via keyboard event listeners
- Manual escape key handling
- Manual body scroll lock
- ~80 lines of custom logic

### After Pattern
- Radix Dialog primitives with composition
- Focus trap, escape, scroll lock handled automatically
- ~40 lines of declarative JSX
- Better accessibility out of the box

### Risk Assessment
| Risk | Mitigation |
|------|------------|
| Focus behavior differs | Radix focus trap is superior; tests may need timing adjustments |
| `aria-labelledby` ID change | Radix generates IDs; update tests to check relationship not specific ID |
| Animation timing | Keep same keyframes, just change triggers to `data-state` |
| Breaking consuming components | Keep identical external API |

### Benefits
- Removes ~40 lines of manual accessibility code
- Better screen reader support via Radix
- Consistent behavior with other Radix components in codebase
- Maintained by Radix team, fewer bugs long-term
