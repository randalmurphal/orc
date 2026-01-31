/**
 * KeyValueEditor - Key-value pair editor for env vars.
 *
 * Allows adding/removing key-value rows. Empty keys are excluded from output.
 * Empty values are included (empty string is valid).
 *
 * Uses internal state for rows to support typing in controlled inputs.
 * Syncs from props only when prop value actually changes (JSON comparison).
 */

import { useState, useCallback, useEffect, useRef } from 'react';
import './KeyValueEditor.css';

export interface KeyValueEditorProps {
	/** Current entries as a key-value map */
	entries: Record<string, string>;
	/** Callback when entries change */
	onChange: (entries: Record<string, string>) => void;
	/** Whether the editor is disabled */
	disabled?: boolean;
}

interface InternalRow {
	id: number;
	key: string;
	value: string;
}

let rowIdCounter = 0;

function entriesToRows(entries: Record<string, string>): InternalRow[] {
	return Object.entries(entries).map(([key, value]) => ({
		id: rowIdCounter++,
		key,
		value,
	}));
}

function rowsToEntries(rows: InternalRow[]): Record<string, string> {
	const result: Record<string, string> = {};
	for (const row of rows) {
		if (row.key.trim()) {
			result[row.key] = row.value;
		}
	}
	return result;
}

export function KeyValueEditor({ entries, onChange, disabled = false }: KeyValueEditorProps) {
	const [rows, setRows] = useState<InternalRow[]>(() => entriesToRows(entries));
	// Track the JSON string of entries to detect external changes
	const lastPropJsonRef = useRef(JSON.stringify(entries));
	// Track whether a change originated internally (to skip prop sync)
	const internalChangeRef = useRef(false);

	// Sync props → internal state only when entries prop changes externally
	useEffect(() => {
		const propJson = JSON.stringify(entries);
		if (propJson !== lastPropJsonRef.current) {
			lastPropJsonRef.current = propJson;
			if (!internalChangeRef.current) {
				setRows(entriesToRows(entries));
			}
			internalChangeRef.current = false;
		}
	}, [entries]);

	const emitChange = useCallback(
		(newRows: InternalRow[]) => {
			setRows(newRows);
			const newEntries = rowsToEntries(newRows);
			internalChangeRef.current = true;
			lastPropJsonRef.current = JSON.stringify(newEntries);
			onChange(newEntries);
		},
		[onChange]
	);

	const handleAdd = useCallback(() => {
		const newRows = [...rows, { id: rowIdCounter++, key: '', value: '' }];
		emitChange(newRows);
	}, [rows, emitChange]);

	const handleKeyChange = useCallback(
		(index: number, newKey: string) => {
			const newRows = rows.map((row, i) =>
				i === index ? { ...row, key: newKey } : row
			);
			emitChange(newRows);
		},
		[rows, emitChange]
	);

	const handleValueChange = useCallback(
		(index: number, newValue: string) => {
			const newRows = rows.map((row, i) =>
				i === index ? { ...row, value: newValue } : row
			);
			emitChange(newRows);
		},
		[rows, emitChange]
	);

	const handleRemove = useCallback(
		(index: number) => {
			const newRows = rows.filter((_, i) => i !== index);
			emitChange(newRows);
		},
		[rows, emitChange]
	);

	return (
		<div className="kv-editor">
			{rows.map((row, index) => (
				<div key={row.id} className="kv-editor__row">
					<input
						type="text"
						className="kv-editor__key"
						value={row.key}
						onChange={(e) => handleKeyChange(index, e.target.value)}
						placeholder="Key"
						disabled={disabled}
					/>
					<input
						type="text"
						className="kv-editor__value"
						value={row.value}
						onChange={(e) => handleValueChange(index, e.target.value)}
						placeholder="Value"
						disabled={disabled}
					/>
					<button
						type="button"
						className="kv-editor__remove"
						onClick={() => handleRemove(index)}
						disabled={disabled}
						aria-label="Remove"
					>
						×
					</button>
				</div>
			))}
			<button
				type="button"
				className="kv-editor__add"
				onClick={handleAdd}
				disabled={disabled}
				aria-label="Add"
			>
				+ Add
			</button>
		</div>
	);
}
