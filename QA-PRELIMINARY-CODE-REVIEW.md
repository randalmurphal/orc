# QA Preliminary Code Review - Settings Page (Iteration 2)

**Date:** 2026-01-28
**Method:** Static Code Analysis
**Status:** ⚠️ CRITICAL BUGS STILL PRESENT

## Executive Summary

Based on code review of the Settings > Slash Commands components, **NONE of the 4 bugs from Iteration 1 have been fixed.**

| Bug ID | Title | Status | Severity | Confidence |
|--------|-------|--------|----------|------------|
| QA-001 | Unsaved changes lost when switching commands | ❌ **NOT FIXED** | CRITICAL | 100% |
| QA-002 | No validation for forward slash (/) | ❌ **NOT FIXED** | HIGH | 100% |
| QA-003 | No validation for spaces | ❌ **NOT FIXED** | HIGH | 100% |
| QA-004 | No max length validation | ❌ **NOT FIXED** | HIGH | 100% |

## Detailed Findings

### QA-001: Unsaved Changes Lost When Switching Commands ❌ NOT FIXED

**Severity:** CRITICAL
**Confidence:** 100%
**Category:** Data Loss

#### Evidence

**File:** `web/src/components/settings/SettingsView.tsx`

**Lines 72-79:**
```typescript
// Update editor content when selection changes (skills already have content)
useEffect(() => {
    if (selectedSkill) {
        setEditorContent(selectedSkill.content);
    } else {
        setEditorContent('');
    }
}, [selectedSkill]);
```

**Problem:**
When the user switches commands, this `useEffect` **immediately overwrites** `editorContent` with the new command's content. There is NO check for unsaved changes.

**Reproduction:**
1. User selects Command A
2. User edits content in editor (types some text)
3. User clicks Command B **without saving**
4. The `useEffect` fires and sets `editorContent = Command B's content`
5. **User's edits to Command A are LOST with NO WARNING**

**Expected Behavior:**
Before switching commands, check if `editorContent !== selectedSkill.content`:
- If unsaved changes exist, show warning dialog: "You have unsaved changes. Discard?"
- User can choose to save, discard, or cancel the switch

**Additional Bug in ConfigEditor:**

**File:** `web/src/components/settings/ConfigEditor.tsx`

**Lines 136-141:**
```typescript
// Track the initial content from when the component first mounts
const [initialContent] = useState(content);
// Track if content has been modified from initial state
const isUnsaved = content !== initialContent;
```

**Problem:**
`initialContent` is set ONCE on component mount and never updated. When the parent passes a new `content` prop (because user switched commands), `initialContent` remains the OLD content. This causes the "Modified" indicator to show incorrectly.

**Example:**
1. Select Command A → `initialContent = "# Command A"`
2. Select Command B → `content = "# Command B"` but `initialContent` STILL `"# Command A"`
3. `isUnsaved = true` (WRONG! - nothing was edited)
4. "Modified" badge appears even though user didn't edit anything

**Fix Required:**
```typescript
useEffect(() => {
    setInitialContent(content);
}, [selectedCommandId]); // Reset when command changes
```

**Impact:**
- Users lose work without warning → **DATA LOSS**
- "Modified" indicator shows false positives → **UX CONFUSION**

---

### QA-002: No Validation for Forward Slash (/) ❌ NOT FIXED

**Severity:** HIGH
**Confidence:** 100%
**Category:** Input Validation / Security

#### Evidence

**File:** `web/src/components/settings/NewCommandModal.tsx`

**Lines 46-50 (ENTIRE validation logic):**
```typescript
const handleCreate = useCallback(async () => {
    if (!name.trim()) {
        toast.error('Name is required');
        return;
    }
    // NO OTHER VALIDATION!
```

**Lines 52-60 (directly sends to API):**
```typescript
setSaving(true);
try {
    const response = await configClient.createSkill({
        name: name.trim(),
        description: description.trim(),
        content: `# ${name.trim()}\n\n<!-- Command content here -->`,
        userInvocable: true,
        scope,
    });
```

**Problem:**
The ONLY validation is checking if the name is empty (line 47). There is NO validation for:
- Forward slash `/`
- Spaces
- Special characters
- Max length
- Path traversal (`../`, `..`)
- Any other constraints

**Reproduction:**
1. Click "New Command"
2. Enter name: `test/command`
3. Click "Create"
4. **Command is created** without any error

**Expected Behavior:**
Show validation error: "Name can only contain letters, numbers, hyphens, and underscores"

**Impact:**
- **Security Risk:** Could enable path traversal or injection attacks
- **Data Integrity:** Invalid command names could break file system operations
- **UX:** Users can create commands that won't work properly

---

### QA-003: No Validation for Spaces ❌ NOT FIXED

**Severity:** HIGH
**Confidence:** 100%
**Category:** Input Validation

#### Evidence

Same as QA-002. The code at lines 46-50 in `NewCommandModal.tsx` only checks for empty string, not spaces.

**Reproduction:**
1. Click "New Command"
2. Enter name: `test command` (with space)
3. Click "Create"
4. **Command is created** without any error

**Expected Behavior:**
Show validation error: "Name can only contain letters, numbers, hyphens, and underscores"

**Impact:**
- **File System Issues:** Spaces in command names could cause issues with shell commands or file paths
- **Inconsistent Behavior:** Some parts of the system might interpret the space differently

---

### QA-004: No Max Length Validation ❌ NOT FIXED

**Severity:** HIGH
**Confidence:** 100%
**Category:** Input Validation

#### Evidence

Same as QA-002. No length validation exists in the code.

**Reproduction:**
1. Click "New Command"
2. Enter name: `aaaaaaaaaa...` (200+ characters)
3. Click "Create"
4. **Command is created** without any error

**Expected Behavior:**
Show validation error: "Name must be 50 characters or less" (or whatever the max is)

**Impact:**
- **UI Issues:** Very long names could break layouts
- **Performance:** Could cause issues with file system operations
- **Database:** Could exceed column limits

---

## Required Fixes

### 1. Add Command Name Validation (QA-002, QA-003, QA-004)

**File:** `web/src/components/settings/NewCommandModal.tsx`

**Lines 46-50 - Replace with:**
```typescript
const COMMAND_NAME_REGEX = /^[a-zA-Z0-9_-]+$/;
const MAX_COMMAND_NAME_LENGTH = 50;

const handleCreate = useCallback(async () => {
    const trimmed = name.trim();

    if (!trimmed) {
        toast.error('Name is required');
        return;
    }

    if (trimmed.length > MAX_COMMAND_NAME_LENGTH) {
        toast.error(`Name must be ${MAX_COMMAND_NAME_LENGTH} characters or less`);
        return;
    }

    if (!COMMAND_NAME_REGEX.test(trimmed)) {
        toast.error('Name can only contain letters, numbers, hyphens, and underscores');
        return;
    }

    // Proceed with creation...
}, [name, description, scope, onCreate, onClose]);
```

### 2. Add Unsaved Changes Warning (QA-001)

**File:** `web/src/components/settings/SettingsView.tsx`

**Lines 72-79 - Replace with:**
```typescript
// Update editor content when selection changes
useEffect(() => {
    const switchCommand = async () => {
        // Check if current command has unsaved changes
        if (selectedSkill && editorContent !== selectedSkill.content) {
            const confirmed = window.confirm(
                'You have unsaved changes. Discard them and switch commands?'
            );
            if (!confirmed) {
                // Revert selection to prevent switch
                // This requires additional state management
                return;
            }
        }

        // Safe to switch
        if (selectedSkill) {
            setEditorContent(selectedSkill.content);
        } else {
            setEditorContent('');
        }
    };

    switchCommand();
}, [selectedSkill]);
```

**Note:** The above is a simplified example. A more robust solution would:
1. Track `previousSelectedId` in state
2. Prevent `selectedId` from updating until user confirms
3. Use a custom modal instead of `window.confirm`

**File:** `web/src/components/settings/ConfigEditor.tsx`

**Lines 136-141 - Replace with:**
```typescript
// Track the initial content, resetting when content prop changes from parent
const [initialContent, setInitialContent] = useState(content);

// Reset initialContent when a new command is selected (content prop changes externally)
// But NOT when user edits (that's internal state)
useEffect(() => {
    // Only reset if this is a new content from parent, not from user editing
    // The parent (SettingsView) updates content when selectedSkill changes
    setInitialContent(content);
}, [selectedSkill?.name]); // Add selectedSkill.name as dependency

// Track if content has been modified from initial state
const isUnsaved = content !== initialContent;
```

**Note:** This requires passing `selectedCommandId` as a prop to `ConfigEditor` or using a different approach.

### 3. Add Browser Refresh Warning (Additional Enhancement)

**File:** `web/src/components/settings/ConfigEditor.tsx`

**Add to component:**
```typescript
useEffect(() => {
    if (isUnsaved) {
        const handler = (e: BeforeUnloadEvent) => {
            e.preventDefault();
            e.returnValue = ''; // Required for Chrome
        };
        window.addEventListener('beforeunload', handler);
        return () => window.removeEventListener('beforeunload', handler);
    }
}, [isUnsaved]);
```

## Code Quality Issues Found

### Issue 1: No Error Handling for Save Failures

**File:** `web/src/components/settings/SettingsView.tsx`
**Lines 106-119:**

```typescript
const handleSave = useCallback(async () => {
    if (!selectedId || !selectedSkill) return;

    try {
        await configClient.updateSkill({
            name: selectedId,
            scope: selectedSkill.scope,
            description: selectedSkill.description,
            content: editorContent,
        });
    } catch (err) {
        console.error('Failed to save command:', err);
        // NO USER FEEDBACK!
    }
}, [selectedId, selectedSkill, editorContent]);
```

**Problem:**
Error is logged to console but NO toast or user feedback. User doesn't know if save succeeded or failed.

**Fix:**
```typescript
try {
    await configClient.updateSkill({ ... });
    toast.success('Command saved');
} catch (err) {
    toast.error('Failed to save command');
}
```

### Issue 2: No Loading State for Save Button

The Save button doesn't show loading state while saving. Users might click multiple times.

**Fix:**
Add `saving` state and disable button while saving.

### Issue 3: No Debouncing for Editor Changes

Every keystroke triggers `onChange` which updates React state. For large files this could be slow.

**Fix:**
Debounce the `onChange` handler.

## Testing Recommendations

### Critical (Must Fix Before Release)

1. ✅ Run automated E2E tests: `./run-qa-iteration2.sh`
2. ✅ Verify all 4 bugs are fixed
3. ✅ Test data loss scenario manually:
   - Edit command A
   - Click command B without saving
   - Verify warning appears

### High Priority

4. ✅ Test all special characters in command names
5. ✅ Test max length validation (50+ characters)
6. ✅ Test browser refresh with unsaved changes

### Medium Priority

7. Test save error handling
8. Test rapid command switching
9. Test mobile viewport (375x667)

## Next Steps

1. **Implement the 3 required fixes** listed above
2. **Run automated tests** to verify fixes: `./run-qa-iteration2.sh`
3. **Manual verification** of critical data loss scenario
4. **Generate final QA report** with screenshots

## Deliverables Required

- [ ] **Validation logic** added to `NewCommandModal.tsx`
- [ ] **Unsaved changes warning** added to `SettingsView.tsx`
- [ ] **InitialContent reset** fixed in `ConfigEditor.tsx`
- [ ] **Test results** from `run-qa-iteration2.sh`
- [ ] **Screenshots** showing:
  - Validation errors for `/`, space, long names
  - Unsaved changes warning dialog
  - Correct "Modified" indicator behavior

## Conclusion

**NONE of the 4 bugs from Iteration 1 have been fixed.** The code is identical to what was analyzed in the previous QA report (QA-REPORT-SLASH-COMMANDS.md).

**Recommendation:**
1. Implement the 3 fixes listed above
2. Run automated E2E tests to verify
3. Do NOT deploy to production without fixing QA-001 (data loss bug)

**Estimated Fix Time:**
- Validation (QA-002, 003, 004): 15 minutes
- Unsaved changes warning (QA-001): 45-60 minutes (requires careful state management)
- Testing: 30 minutes

**Total:** ~2 hours including testing

---

**QA Status:** ❌ FAILED - Critical bugs still present, not ready for production
