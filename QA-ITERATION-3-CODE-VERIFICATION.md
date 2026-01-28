# QA Iteration 3 - Code Verification Report

**Date**: 2026-01-28
**Task**: TASK-616
**Method**: Static code analysis (Playwright MCP tools unavailable)
**Status**: All 4 findings remain UNRESOLVED

## Verification Summary

| Finding | Status | Confidence | Evidence |
|---------|--------|------------|----------|
| QA-002: Forward slash validation | ❌ STILL_PRESENT | 95% | No validation code exists |
| QA-003: Spaces validation | ❌ STILL_PRESENT | 95% | No validation code exists |
| QA-004: Length validation | ❌ STILL_PRESENT | 95% | No validation code exists |
| QA-005: Modified indicator bug | ❌ STILL_PRESENT | 95% | useState not updated on prop change |

---

## QA-002: Forward Slash Validation - STILL_PRESENT

**File**: `web/src/components/settings/NewCommandModal.tsx`
**Lines**: 46-50

### Current Code
```typescript
const handleCreate = useCallback(async () => {
    if (!name.trim()) {
        toast.error('Name is required');
        return;
    }
    // No other validation - proceeds directly to createSkill
    setSaving(true);
    try {
        const response = await configClient.createSkill({
            name: name.trim(),  // ← Accepts ANY non-empty string
            // ...
        });
    }
    // ...
}, [name, description, scope, onCreate, onClose]);
```

### Evidence
- **Only validation**: Empty string check
- **Missing**: Regex check for forward slashes
- **Impact**: Command names like `test/command` will be accepted, causing filesystem path issues

### Expected Fix
```typescript
if (!name.trim()) {
    toast.error('Name is required');
    return;
}
// ADD THIS:
if (!/^[a-zA-Z0-9_-]+$/.test(name.trim())) {
    toast.error('Command names can only contain letters, numbers, hyphens, and underscores');
    return;
}
```

---

## QA-003: Spaces Validation - STILL_PRESENT

**File**: `web/src/components/settings/NewCommandModal.tsx`
**Lines**: 46-50 (same location as QA-002)

### Current Code
Same as QA-002 - no regex validation exists.

### Evidence
- Command names with spaces (e.g., `test command`) are accepted
- No validation prevents this
- Same fix as QA-002 resolves this issue

### Impact
- Commands with spaces unusable in CLI: `/test command` would be parsed as `/test` with argument `command`
- User confusion and support tickets

---

## QA-004: Length Validation - STILL_PRESENT

**File**: `web/src/components/settings/NewCommandModal.tsx`
**Lines**: 46-50 (same location as QA-002, QA-003)

### Current Code
Same as QA-002/QA-003 - no length validation exists.

### Evidence
- No maximum length check before `createSkill()` call
- 200+ character names accepted without error
- Could cause filesystem issues (filename length limits) and UI overflow

### Expected Fix
```typescript
if (!name.trim()) {
    toast.error('Name is required');
    return;
}
// ADD THIS:
if (name.trim().length > 50) {
    toast.error('Command name must be 50 characters or less');
    return;
}
if (!/^[a-zA-Z0-9_-]+$/.test(name.trim())) {
    toast.error('Command names can only contain letters, numbers, hyphens, and underscores');
    return;
}
```

---

## QA-005: Modified Indicator Bug - STILL_PRESENT

**File**: `web/src/components/settings/ConfigEditor.tsx`
**Lines**: 136, 141

### Current Code
```typescript
export function ConfigEditor({
    filePath,
    content,  // ← Prop changes when switching commands
    onChange,
    onSave,
    language = 'markdown',
}: ConfigEditorProps) {
    // BUG: useState only runs on first mount
    const [initialContent] = useState(content);  // Line 136

    // ...

    // This comparison breaks when content prop changes
    const isUnsaved = content !== initialContent;  // Line 141
```

### Root Cause
`useState(content)` only captures the initial value on first component mount. When the parent component changes the `content` prop (by switching commands), `initialContent` remains frozen at the old value.

**Flow:**
1. User selects Command A → `initialContent = "A's content"`
2. User selects Command B → `content` prop updates to "B's content"
3. `initialContent` stays as "A's content" (useState doesn't re-run)
4. `isUnsaved = ("B's content" !== "A's content")` → `true`
5. "Modified" indicator appears incorrectly

### Expected Fix
```typescript
// Option 1: Use useEffect to sync initialContent
const [initialContent, setInitialContent] = useState(content);

useEffect(() => {
    setInitialContent(content);
}, [content]);

// Option 2: Use useRef instead
const initialContentRef = useRef(content);
useEffect(() => {
    initialContentRef.current = content;
}, [content]);
const isUnsaved = content !== initialContentRef.current;
```

---

## Recommended Action

**DO NOT MERGE** until all 4 issues are resolved.

### Fix Priority
1. **QA-002, QA-003, QA-004** (HIGH) - Add validation in one go (~15 min)
2. **QA-005** (MEDIUM) - Fix state management (~5 min)

### Total Time to Fix
- Implementation: 20 minutes
- Testing: 15 minutes
- **Total**: ~35 minutes

### Why These Matter
- **Validation bugs**: Users will create broken commands, file tickets, get frustrated
- **Modified indicator bug**: Users lose trust in the save system, save unnecessarily, or lose work thinking changes were saved
- All are simple fixes with high user impact

---

## Testing Limitation Note

This verification was conducted via **static code analysis** because Playwright MCP tools were not available in the testing environment. The findings are based on:

1. Direct inspection of source code
2. Understanding of React component lifecycle
3. Validation logic (or lack thereof) in the codebase

**Recommendation**: After fixes are implemented, conduct live browser testing to verify:
- Validation messages appear correctly
- Modified indicator works as expected
- Mobile viewport (375x667) functions properly
- No console errors occur during workflows
