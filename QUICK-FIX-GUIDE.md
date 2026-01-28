# Quick Fix Guide - Settings Page Validation

**Time to Fix**: ~45 minutes
**Difficulty**: Easy
**Files to Modify**: 2

---

## Fix 1: Add Validation to NewCommandModal (QA-002, QA-003, QA-004)

### File: `web/src/components/settings/NewCommandModal.tsx`

### Current Code (Lines 46-50):
```typescript
const handleCreate = useCallback(async () => {
    if (!name.trim()) {
        toast.error('Name is required');
        return;
    }

    setSaving(true);
    // ... rest of creation logic
```

### Fixed Code:
```typescript
const handleCreate = useCallback(async () => {
    const trimmed = name.trim();

    // Validation: Required
    if (!trimmed) {
        toast.error('Name is required');
        return;
    }

    // Validation: No spaces
    if (/\s/.test(trimmed)) {
        toast.error('Command names cannot contain spaces');
        return;
    }

    // Validation: No forward slashes
    if (trimmed.includes('/')) {
        toast.error('Command names cannot contain forward slashes');
        return;
    }

    // Validation: Maximum length
    if (trimmed.length > 50) {
        toast.error('Command name must be 50 characters or less');
        return;
    }

    // Optional: Only allow alphanumeric, hyphens, underscores
    if (!/^[a-zA-Z0-9_-]+$/.test(trimmed)) {
        toast.error('Command names can only contain letters, numbers, hyphens, and underscores');
        return;
    }

    setSaving(true);
    try {
        const response = await configClient.createSkill({
            name: trimmed, // Use trimmed instead of name.trim()
            description: description.trim(),
            content: `# ${trimmed}\n\n<!-- Command content here -->`,
            userInvocable: true,
            scope,
        });
        // ... rest of creation logic
```

**Changes Made**:
1. Store `name.trim()` in `trimmed` variable
2. Add 4 validation checks with clear error messages
3. Use `trimmed` consistently throughout

**Testing**:
```bash
# After fix, test these cases:
# 1. Try "test/command" → Should show error
# 2. Try "test command" → Should show error
# 3. Try 200 'a' chars → Should show error
# 4. Try "valid-command" → Should succeed
```

---

## Fix 2: Fix Modified Indicator (QA-005)

### File: `web/src/components/settings/ConfigEditor.tsx`

### Current Code (Line 136):
```typescript
// Track the initial content from when the component first mounts
const [initialContent] = useState(content);
```

### Fixed Code:
```typescript
import {
    type ChangeEvent,
    type KeyboardEvent,
    useCallback,
    useEffect,  // ← ADD THIS IMPORT
    useMemo,
    useRef,
    useState,
} from 'react';

// ... other imports ...

export function ConfigEditor({
    filePath,
    content,
    onChange,
    onSave,
    language = 'markdown',
}: ConfigEditorProps) {
    // Track the initial content - update when file changes
    const [initialContent, setInitialContent] = useState(content);
    const textareaRef = useRef<HTMLTextAreaElement>(null);
    const highlightRef = useRef<HTMLDivElement>(null);

    // Reset initial content when filePath changes (new command selected)
    useEffect(() => {
        setInitialContent(content);
    }, [filePath, content]);

    // Track if content has been modified from initial state
    const isUnsaved = content !== initialContent;

    // ... rest of component
```

**Changes Made**:
1. Import `useEffect` from React
2. Change `useState(content)` to store setter function
3. Add `useEffect` that resets `initialContent` when `filePath` or `content` changes
4. This ensures "Modified" indicator resets when switching commands

**Testing**:
```bash
# After fix, test this case:
# 1. Click first command
# 2. Don't edit anything
# 3. Click second command
# 4. Verify NO "Modified" indicator shown
```

---

## Verification Checklist

After implementing fixes:

### Manual Testing
- [ ] Try creating command with slash → Error shown
- [ ] Try creating command with spaces → Error shown
- [ ] Try creating command with 200 chars → Error shown
- [ ] Switch between commands without editing → No "Modified" shown
- [ ] Create valid command "test-cmd-123" → Success

### Run Automated Tests
```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-616
./RUN-QA-ITERATION-3.sh
```

**Expected Output After Fixes**:
```
✅ QA-002 (high): Forward slash validation
   Status: FIXED (confidence: 90%)

✅ QA-003 (high): Spaces validation
   Status: FIXED (confidence: 90%)

✅ QA-004 (high): Length validation
   Status: FIXED (confidence: 90%)

✅ QA-005 (medium): Modified indicator bug
   Status: FIXED (confidence: 85%)

✅ All previous issues have been fixed!
```

---

## Optional: Extract to Reusable Function

For cleaner code, extract validation logic:

### Create: `web/src/lib/validation.ts`
```typescript
export interface ValidationResult {
    valid: boolean;
    error?: string;
}

export function validateCommandName(name: string): ValidationResult {
    const trimmed = name.trim();

    if (!trimmed) {
        return { valid: false, error: 'Name is required' };
    }

    if (/\s/.test(trimmed)) {
        return { valid: false, error: 'Command names cannot contain spaces' };
    }

    if (trimmed.includes('/')) {
        return { valid: false, error: 'Command names cannot contain forward slashes' };
    }

    if (trimmed.length > 50) {
        return { valid: false, error: 'Command name must be 50 characters or less' };
    }

    if (!/^[a-zA-Z0-9_-]+$/.test(trimmed)) {
        return {
            valid: false,
            error: 'Command names can only contain letters, numbers, hyphens, and underscores',
        };
    }

    return { valid: true };
}
```

### Update `NewCommandModal.tsx`:
```typescript
import { validateCommandName } from '@/lib/validation';

const handleCreate = useCallback(async () => {
    const validation = validateCommandName(name);

    if (!validation.valid) {
        toast.error(validation.error!);
        return;
    }

    setSaving(true);
    try {
        const response = await configClient.createSkill({
            name: name.trim(),
            description: description.trim(),
            content: `# ${name.trim()}\n\n<!-- Command content here -->`,
            userInvocable: true,
            scope,
        });
        // ... rest
```

**Benefits**:
- Reusable validation logic
- Easier to test
- Single source of truth for validation rules
- Can be used in other components

---

## Unit Tests (Optional but Recommended)

### Create: `web/src/lib/validation.test.ts`
```typescript
import { describe, it, expect } from 'vitest';
import { validateCommandName } from './validation';

describe('validateCommandName', () => {
    it('accepts valid command names', () => {
        expect(validateCommandName('valid-command')).toEqual({ valid: true });
        expect(validateCommandName('test_123')).toEqual({ valid: true });
        expect(validateCommandName('my-cmd')).toEqual({ valid: true });
    });

    it('rejects empty names', () => {
        expect(validateCommandName('')).toEqual({
            valid: false,
            error: 'Name is required',
        });
        expect(validateCommandName('   ')).toEqual({
            valid: false,
            error: 'Name is required',
        });
    });

    it('rejects names with spaces', () => {
        expect(validateCommandName('test command')).toEqual({
            valid: false,
            error: 'Command names cannot contain spaces',
        });
    });

    it('rejects names with slashes', () => {
        expect(validateCommandName('test/command')).toEqual({
            valid: false,
            error: 'Command names cannot contain forward slashes',
        });
    });

    it('rejects names exceeding 50 characters', () => {
        const longName = 'a'.repeat(51);
        expect(validateCommandName(longName)).toEqual({
            valid: false,
            error: 'Command name must be 50 characters or less',
        });
    });

    it('rejects names with invalid characters', () => {
        expect(validateCommandName('test@command')).toEqual({
            valid: false,
            error: 'Command names can only contain letters, numbers, hyphens, and underscores',
        });
    });
});
```

Run tests:
```bash
cd web
bun run test validation.test.ts
```

---

## Summary

**2 Files to Change**:
1. `web/src/components/settings/NewCommandModal.tsx` - Add validation (4 checks)
2. `web/src/components/settings/ConfigEditor.tsx` - Fix state management (1 useEffect)

**Time Estimate**:
- Validation fix: 15 minutes
- State management fix: 5 minutes
- Testing: 15 minutes
- Unit tests (optional): 10 minutes
- **Total**: 30-45 minutes

**Risk Level**: LOW
- Simple validation logic
- Standard React patterns
- No external dependencies
- Easy to test

**Impact**: HIGH
- Fixes 4 user-facing bugs
- Improves data quality
- Better user experience
- Prevents support tickets

---

## Questions?

If anything is unclear:
1. Review `QA-ITERATION-3-CODE-ANALYSIS.md` for detailed explanations
2. Review `QA-ITERATION-3-FINAL-SUMMARY.md` for full context
3. Run `./RUN-QA-ITERATION-3.sh` to see current test results
4. Check screenshots in `web/qa-screenshots-iter3/` for visual evidence

**After fixes are implemented**, re-run the test suite to confirm all issues are resolved!
