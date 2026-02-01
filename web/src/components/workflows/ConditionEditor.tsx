/**
 * ConditionEditor - Visual condition builder with raw JSON mode
 *
 * Implements TASK-694: Condition editor UI in phase inspector
 *
 * Features:
 * - Visual builder with field/operator/value rows
 * - Compound condition support (all/any wrapper)
 * - Logic toggle between all/any when 2+ conditions
 * - Raw JSON editor mode with validation
 * - Fallback to raw mode for malformed/nested conditions
 * - Known value dropdowns for task.weight/task.category
 * - Read-only mode for built-in workflows
 */

import { useState, useCallback, useEffect } from 'react';
import './ConditionEditor.css';

// ─── Types ───────────────────────────────────────────────────────────────────

export interface ConditionEditorProps {
	/** JSON string representing the condition */
	condition: string;
	/** Called when condition changes (receives JSON string) */
	onChange: (condition: string) => void;
	/** When true, all controls are disabled */
	disabled?: boolean;
}

interface SimpleCondition {
	field: string;
	op: string;
	value?: string | string[];
}

interface CompoundCondition {
	all?: SimpleCondition[];
	any?: SimpleCondition[];
}

type Condition = SimpleCondition | CompoundCondition;

interface ConditionRow {
	id: string;
	field: string;
	op: string;
	value: string | string[];
}

// ─── Constants ───────────────────────────────────────────────────────────────

const OPERATORS = [
	{ value: 'eq', label: 'equals' },
	{ value: 'neq', label: 'not equals' },
	{ value: 'in', label: 'is one of' },
	{ value: 'contains', label: 'contains' },
	{ value: 'exists', label: 'exists' },
	{ value: 'gt', label: 'greater than' },
	{ value: 'lt', label: 'less than' },
];

const KNOWN_FIELDS = [
	{ value: 'task.weight', label: 'Task Weight' },
	{ value: 'task.category', label: 'Task Category' },
	{ value: 'task.priority', label: 'Task Priority' },
];

const WEIGHT_VALUES = ['trivial', 'small', 'medium', 'large'];
const CATEGORY_VALUES = ['feature', 'bug', 'refactor', 'chore', 'docs', 'test'];
const PRIORITY_VALUES = ['critical', 'high', 'normal', 'low'];

// ─── Helpers ─────────────────────────────────────────────────────────────────

let rowIdCounter = 0;
function generateRowId(): string {
	return `row-${++rowIdCounter}`;
}

function isSimpleCondition(cond: Condition): cond is SimpleCondition {
	return 'field' in cond && 'op' in cond;
}

function isCompoundCondition(cond: Condition): cond is CompoundCondition {
	return 'all' in cond || 'any' in cond;
}

function hasNestedCompound(conditions: SimpleCondition[]): boolean {
	return conditions.some((c) => 'all' in c || 'any' in c);
}

function parseCondition(json: string): {
	rows: ConditionRow[];
	logic: 'all' | 'any';
	useRawMode: boolean;
	rawValue: string;
} {
	if (!json || json.trim() === '') {
		return { rows: [], logic: 'all', useRawMode: false, rawValue: '' };
	}

	try {
		const parsed = JSON.parse(json) as Condition;

		if (isSimpleCondition(parsed)) {
			return {
				rows: [
					{
						id: generateRowId(),
						field: parsed.field,
						op: parsed.op,
						value: parsed.value ?? '',
					},
				],
				logic: 'all',
				useRawMode: false,
				rawValue: json,
			};
		}

		if (isCompoundCondition(parsed)) {
			const logic: 'all' | 'any' = 'all' in parsed ? 'all' : 'any';
			const conditions = parsed.all ?? parsed.any ?? [];

			// Check for nested compound conditions - fall back to raw mode
			if (hasNestedCompound(conditions as SimpleCondition[])) {
				return { rows: [], logic: 'all', useRawMode: true, rawValue: json };
			}

			const rows: ConditionRow[] = (conditions as SimpleCondition[]).map((c) => ({
				id: generateRowId(),
				field: c.field,
				op: c.op,
				value: c.value ?? '',
			}));

			return { rows, logic, useRawMode: false, rawValue: json };
		}

		// Unknown structure - raw mode
		return { rows: [], logic: 'all', useRawMode: true, rawValue: json };
	} catch {
		// Malformed JSON - raw mode
		return { rows: [], logic: 'all', useRawMode: true, rawValue: json };
	}
}

function serializeCondition(rows: ConditionRow[], logic: 'all' | 'any'): string {
	// Filter out incomplete rows
	const completeRows = rows.filter((r) => r.field && r.op);

	if (completeRows.length === 0) {
		return '';
	}

	const conditions: SimpleCondition[] = completeRows.map((r) => {
		const cond: SimpleCondition = { field: r.field, op: r.op };
		// Only add value if operator isn't "exists"
		if (r.op !== 'exists') {
			cond.value = r.value;
		}
		return cond;
	});

	if (conditions.length === 1) {
		return JSON.stringify(conditions[0]);
	}

	return JSON.stringify({ [logic]: conditions });
}

function getKnownValues(field: string): string[] | null {
	switch (field) {
		case 'task.weight':
			return WEIGHT_VALUES;
		case 'task.category':
			return CATEGORY_VALUES;
		case 'task.priority':
			return PRIORITY_VALUES;
		default:
			return null;
	}
}

// ─── Component ───────────────────────────────────────────────────────────────

export function ConditionEditor({
	condition,
	onChange,
	disabled = false,
}: ConditionEditorProps) {
	// Use useState initializer functions for initial parsing (runs once on mount)
	const [rows, setRows] = useState<ConditionRow[]>(() => parseCondition(condition).rows);
	const [logic, setLogic] = useState<'all' | 'any'>(() => parseCondition(condition).logic);
	const [rawMode, setRawMode] = useState(() => parseCondition(condition).useRawMode);
	const [rawValue, setRawValue] = useState(() => condition || '');
	const [rawError, setRawError] = useState<string | null>(null);

	// Reset state when condition prop changes (controlled component)
	useEffect(() => {
		const parsed = parseCondition(condition);
		setRows(parsed.rows);
		setLogic(parsed.logic);
		setRawMode(parsed.useRawMode);
		setRawValue(condition || '');
		setRawError(null);
	}, [condition]);

	const emitChange = useCallback(
		(newRows: ConditionRow[], newLogic: 'all' | 'any') => {
			const json = serializeCondition(newRows, newLogic);
			onChange(json);
		},
		[onChange],
	);

	const handleAddRow = useCallback(() => {
		const newRow: ConditionRow = {
			id: generateRowId(),
			field: '',
			op: 'eq',
			value: '',
		};
		const newRows = [...rows, newRow];
		setRows(newRows);
		// Don't emit change yet - incomplete row
	}, [rows]);

	const handleRemoveRow = useCallback(
		(rowId: string) => {
			const newRows = rows.filter((r) => r.id !== rowId);
			setRows(newRows);
			emitChange(newRows, logic);
		},
		[rows, logic, emitChange],
	);

	const handleRowChange = useCallback(
		(rowId: string, field: keyof ConditionRow, value: string | string[]) => {
			const newRows = rows.map((r) => {
				if (r.id !== rowId) return r;
				const updated = { ...r, [field]: value };
				// Reset value when operator changes to "in" or from "in"
				if (field === 'op') {
					if (value === 'in' && !Array.isArray(r.value)) {
						updated.value = [];
					} else if (value !== 'in' && Array.isArray(r.value)) {
						updated.value = '';
					}
					// Clear value for "exists" operator
					if (value === 'exists') {
						updated.value = '';
					}
				}
				// Reset value when field changes
				if (field === 'field') {
					updated.value = updated.op === 'in' ? [] : '';
				}
				return updated;
			});
			setRows(newRows);
			emitChange(newRows, logic);
		},
		[rows, logic, emitChange],
	);

	const handleLogicChange = useCallback(
		(newLogic: 'all' | 'any') => {
			setLogic(newLogic);
			emitChange(rows, newLogic);
		},
		[rows, emitChange],
	);

	const handleToggleRawMode = useCallback(() => {
		if (rawMode) {
			// Switching from raw to visual - validate first
			if (rawError) {
				// Don't allow switch with invalid JSON
				return;
			}
			const parsed = parseCondition(rawValue);
			if (parsed.useRawMode) {
				// Can't switch - nested or unsupported structure
				return;
			}
			setRows(parsed.rows);
			setLogic(parsed.logic);
			setRawMode(false);
		} else {
			// Switching from visual to raw
			const json = serializeCondition(rows, logic);
			setRawValue(json ? JSON.stringify(JSON.parse(json), null, 2) : '');
			setRawMode(true);
			setRawError(null);
		}
	}, [rawMode, rawValue, rawError, rows, logic]);

	const handleRawValueChange = useCallback(
		(value: string) => {
			setRawValue(value);
		},
		[],
	);

	const handleRawValueBlur = useCallback(
		(e: React.FocusEvent<HTMLTextAreaElement>) => {
			const value = e.target.value;
			if (!value.trim()) {
				setRawError(null);
				onChange('');
				return;
			}
			try {
				JSON.parse(value);
				setRawError(null);
				onChange(value);
			} catch {
				setRawError('Invalid JSON');
			}
		},
		[onChange],
	);

	const handleArrayValueAdd = useCallback(
		(rowId: string, newValue: string) => {
			const row = rows.find((r) => r.id === rowId);
			if (!row || !Array.isArray(row.value)) return;
			if (!newValue.trim() || row.value.includes(newValue.trim())) return;

			const updatedValue = [...row.value, newValue.trim()];
			handleRowChange(rowId, 'value', updatedValue);
		},
		[rows, handleRowChange],
	);

	const handleArrayValueRemove = useCallback(
		(rowId: string, valueToRemove: string) => {
			const row = rows.find((r) => r.id === rowId);
			if (!row || !Array.isArray(row.value)) return;

			const updatedValue = row.value.filter((v) => v !== valueToRemove);
			handleRowChange(rowId, 'value', updatedValue);
		},
		[rows, handleRowChange],
	);

	// ─── Render Raw JSON Mode ────────────────────────────────────────────────

	if (rawMode) {
		return (
			<div className="condition-editor condition-editor--raw">
				{!disabled && (
					<div className="condition-editor__mode-toggle">
						<button
							type="button"
							className="condition-editor__mode-btn"
							onClick={handleToggleRawMode}
							aria-label="Visual builder"
						>
							Visual
						</button>
						<button
							type="button"
							className="condition-editor__mode-btn condition-editor__mode-btn--active"
							aria-label="JSON editor"
						>
							JSON
						</button>
					</div>
				)}
				<textarea
					className={`condition-editor__textarea ${rawError ? 'condition-editor__textarea--error' : ''}`}
					value={rawValue}
					onChange={(e) => handleRawValueChange(e.target.value)}
					onBlur={handleRawValueBlur}
					disabled={disabled}
					rows={6}
					placeholder="Enter condition JSON..."
				/>
				{rawError && (
					<div className="condition-editor__error">{rawError}</div>
				)}
			</div>
		);
	}

	// ─── Render Visual Builder ───────────────────────────────────────────────

	const showLogicToggle = rows.length >= 2 && !disabled;

	return (
		<div className="condition-editor condition-editor--visual">
			{!disabled && rows.length > 0 && (
				<div className="condition-editor__mode-toggle">
					<button
						type="button"
						className="condition-editor__mode-btn condition-editor__mode-btn--active"
						aria-label="Visual builder"
					>
						Visual
					</button>
					<button
						type="button"
						className="condition-editor__mode-btn"
						onClick={handleToggleRawMode}
						aria-label="JSON editor"
					>
						JSON
					</button>
				</div>
			)}

			{showLogicToggle && (
				<div className="condition-editor__logic-toggle" role="radiogroup" aria-label="Logic mode">
					<label className="condition-editor__logic-option">
						<input
							type="radio"
							name="logic-mode"
							value="all"
							checked={logic === 'all'}
							onChange={() => handleLogicChange('all')}
							disabled={disabled}
							aria-label="All"
						/>
						<span>All</span>
					</label>
					<label className="condition-editor__logic-option">
						<input
							type="radio"
							name="logic-mode"
							value="any"
							checked={logic === 'any'}
							onChange={() => handleLogicChange('any')}
							disabled={disabled}
							aria-label="Any"
						/>
						<span>Any</span>
					</label>
				</div>
			)}

			{/* Disabled compound conditions display logic indicator */}
			{disabled && rows.length >= 2 && (
				<div className="condition-editor__logic-toggle" role="radiogroup" aria-label="Logic mode">
					<label className="condition-editor__logic-option">
						<input
							type="radio"
							name="logic-mode"
							value="all"
							checked={logic === 'all'}
							disabled={true}
							aria-label="All"
						/>
						<span>All</span>
					</label>
					<label className="condition-editor__logic-option">
						<input
							type="radio"
							name="logic-mode"
							value="any"
							checked={logic === 'any'}
							disabled={true}
							aria-label="Any"
						/>
						<span>Any</span>
					</label>
				</div>
			)}

			<div className="condition-editor__rows">
				{rows.map((row) => (
					<ConditionRowComponent
						key={row.id}
						row={row}
						disabled={disabled}
						onFieldChange={(value) => handleRowChange(row.id, 'field', value)}
						onOpChange={(value) => handleRowChange(row.id, 'op', value)}
						onValueChange={(value) => handleRowChange(row.id, 'value', value)}
						onArrayValueAdd={(value) => handleArrayValueAdd(row.id, value)}
						onArrayValueRemove={(value) => handleArrayValueRemove(row.id, value)}
						onRemove={() => handleRemoveRow(row.id)}
					/>
				))}
			</div>

			{!disabled && (
				<button
					type="button"
					className="condition-editor__add-btn"
					onClick={handleAddRow}
					aria-label="Add condition"
				>
					+ Add condition
				</button>
			)}
		</div>
	);
}

// ─── Condition Row Component ─────────────────────────────────────────────────

interface ConditionRowComponentProps {
	row: ConditionRow;
	disabled: boolean;
	onFieldChange: (value: string) => void;
	onOpChange: (value: string) => void;
	onValueChange: (value: string | string[]) => void;
	onArrayValueAdd: (value: string) => void;
	onArrayValueRemove: (value: string) => void;
	onRemove: () => void;
}

function ConditionRowComponent({
	row,
	disabled,
	onFieldChange,
	onOpChange,
	onValueChange,
	onArrayValueAdd,
	onArrayValueRemove,
	onRemove,
}: ConditionRowComponentProps) {
	const [tagInput, setTagInput] = useState('');
	const knownValues = getKnownValues(row.field);
	const showValueInput = row.op !== 'exists';
	const isArrayOperator = row.op === 'in';

	const handleTagKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
		if (e.key === 'Enter') {
			e.preventDefault();
			onArrayValueAdd(tagInput);
			setTagInput('');
		}
	};

	return (
		<div className="condition-editor__row" role="group">
			{/* Field Select */}
			<div className="condition-editor__field-group">
				<label className="condition-editor__label" htmlFor={`field-${row.id}`}>
					Field
				</label>
				<select
					id={`field-${row.id}`}
					className="condition-editor__select"
					value={row.field}
					onChange={(e) => onFieldChange(e.target.value)}
					disabled={disabled}
					aria-label="Field"
				>
					<option value="">Select field...</option>
					{KNOWN_FIELDS.map((f) => (
						<option key={f.value} value={f.value}>
							{f.label}
						</option>
					))}
					{/* Show custom field if it's not in known fields */}
					{row.field && !KNOWN_FIELDS.some((f) => f.value === row.field) && (
						<option value={row.field}>{row.field}</option>
					)}
				</select>
			</div>

			{/* Operator Select */}
			<div className="condition-editor__field-group">
				<label className="condition-editor__label" htmlFor={`op-${row.id}`}>
					Operator
				</label>
				<select
					id={`op-${row.id}`}
					className="condition-editor__select"
					value={row.op}
					onChange={(e) => onOpChange(e.target.value)}
					disabled={disabled}
					aria-label="Operator"
				>
					{OPERATORS.map((op) => (
						<option key={op.value} value={op.value}>
							{op.label}
						</option>
					))}
				</select>
			</div>

			{/* Value Input */}
			{showValueInput && (
				<div className="condition-editor__field-group">
					<label className="condition-editor__label" htmlFor={`value-${row.id}`}>
						Value
					</label>
					{isArrayOperator ? (
						<div className="condition-editor__array-input">
							<div className="condition-editor__array-values">
								{Array.isArray(row.value) &&
									row.value.map((v) => (
										<span key={v} className="condition-editor__array-tag">
											{v}
											{!disabled && (
												<button
													type="button"
													className="condition-editor__array-tag-remove"
													onClick={() => onArrayValueRemove(v)}
													aria-label={`Remove ${v}`}
												>
													×
												</button>
											)}
										</span>
									))}
							</div>
							<input
								type="text"
								className="condition-editor__input"
								value={tagInput}
								onChange={(e) => setTagInput(e.target.value)}
								onKeyDown={handleTagKeyDown}
								disabled={disabled}
								placeholder="Add value..."
								aria-label="Value"
							/>
						</div>
					) : (
						<>
							<input
								type="text"
								id={`value-${row.id}`}
								className="condition-editor__input"
								value={typeof row.value === 'string' ? row.value : ''}
								onChange={(e) => onValueChange(e.target.value)}
								disabled={disabled}
								placeholder="Enter value..."
								aria-label="Value"
								list={knownValues ? `values-${row.id}` : undefined}
							/>
							{knownValues && (
								<datalist id={`values-${row.id}`}>
									{knownValues.map((v) => (
										<option key={v} value={v}>
											{v}
										</option>
									))}
								</datalist>
							)}
						</>
					)}
				</div>
			)}

			{/* Remove Button */}
			{!disabled && (
				<button
					type="button"
					className="condition-editor__remove-btn"
					onClick={onRemove}
					aria-label="Remove"
				>
					Remove
				</button>
			)}
		</div>
	);
}

export type { ConditionRow, SimpleCondition, CompoundCondition };
