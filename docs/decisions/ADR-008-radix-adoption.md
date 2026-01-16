# ADR-008: Radix UI Adoption

**Status**: Accepted
**Date**: 2026-01-15

---

## Context

The orc web UI needs accessible component primitives for interactive elements like dialogs, dropdown menus, selects, tabs, and tooltips. These components require complex behavior:

| Requirement | Complexity |
|-------------|------------|
| Focus management | High - must trap focus in modals, return on close |
| Keyboard navigation | High - arrow keys, escape, enter behaviors |
| ARIA attributes | Medium - roles, states, properties |
| Screen reader announcements | Medium - live regions, label associations |
| Click-outside handling | Medium - portals, event propagation |

Building these from scratch is error-prone and time-consuming.

## Decision

**Adopt Radix UI for accessible component primitives.**

Installed packages:
- `@radix-ui/react-dialog` - Modals, alerts
- `@radix-ui/react-dropdown-menu` - Context menus, action menus
- `@radix-ui/react-select` - Custom select inputs
- `@radix-ui/react-tabs` - Tab panels
- `@radix-ui/react-tooltip` - Hover tooltips

Additional packages for extensibility:
- `@radix-ui/react-popover` - Generic popovers
- `@radix-ui/react-slot` - Component composition
- `@radix-ui/react-toast` - Toast notifications

## Options Considered

| Option | Pros | Cons |
|--------|------|------|
| Native HTML only | No deps, best perf | Missing focus trap, keyboard nav |
| Headless UI | Tailwind integration | React 19 compatibility issues |
| React Aria | Adobe backing, comprehensive | Heavy, complex API |
| **Radix UI** | Unstyled, React 19 support, active | Requires styling |

## Rationale

### Why Radix?

1. **Unstyled by default** - Works with existing CSS, no style conflicts
2. **React 19 compatible** - Tested with React 19, no deprecation warnings
3. **TypeScript-first** - Full type coverage, great DX
4. **Component-level tree-shaking** - Only pay for what you use
5. **Active maintenance** - Regular updates, responsive maintainers

### Portal Behavior

All Radix overlay components (Dialog, Dropdown, Select, Tooltip, Popover) portal to `document.body` by default. This:
- Prevents z-index stacking issues
- Escapes CSS overflow:hidden parents
- Matches existing Modal.tsx behavior

### Styling Approach

Components expose `data-*` attributes for CSS styling:

```css
[data-state='open'] { /* open state */ }
[data-state='closed'] { /* closed state */ }
[data-highlighted] { /* keyboard/hover focus */ }
[data-disabled] { /* disabled state */ }
```

Global animations defined in `index.css` apply to all Radix components.

## Active Usage

| Component | Radix Package | Usage |
|-----------|---------------|-------|
| `TaskCard` | `@radix-ui/react-dropdown-menu` | Quick menu for queue/priority changes |
| `Modal` | `@radix-ui/react-dialog` | Base modal dialogs |

## Consequences

**Positive**:
- Accessibility handled correctly out of box
- Focus management automatic
- Keyboard navigation works without custom code
- Screen readers properly announce state changes
- Reduces custom component complexity

**Negative**:
- Additional dependency (~3-8KB gzipped per component used)
- Learning curve for Radix patterns
- Must style everything manually

**Mitigation**: Bundle impact is per-component; wrapper components can standardize styling patterns.

## References

- [Radix UI Documentation](https://www.radix-ui.com/primitives/docs/overview/introduction)
- [WAI-ARIA Authoring Practices](https://www.w3.org/WAI/ARIA/apg/)
