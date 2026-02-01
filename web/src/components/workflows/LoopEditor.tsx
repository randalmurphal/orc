/**
 * LoopEditor - Configure phase loops with condition-based iteration
 *
 * Implements TASK-695: Loop editor UI in phase inspector
 *
 * Features:
 * - Loop back to phase dropdown (prior phases only)
 * - Condition editor for loop trigger (reuses ConditionEditor)
 * - Max loops input (1-10)
 * - JSON serialization to loop_config field
 */

import { useState, useCallback, useEffect, useMemo } from 'react';
import { ConditionEditor } from './ConditionEditor';
import './LoopEditor.css';

// ─── Types ───────────────────────────────────────────────────────────────────

export interface LoopEditorProps {
	/** JSON string representing the loop configuration */
	loopConfig: string;
	/** Called when loop config changes (receives JSON string) */
	onChange: (loopConfig: string) => void;
	/** Phase IDs that come before this phase (valid loop targets) */
	priorPhases: string[];
	/** When true, all controls are disabled */
	disabled?: boolean;
}

interface LoopConfig {
	loop_to_phase: string;
	condition: unknown;
	max_loops?: number;
	max_iterations?: number; // Legacy field
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

function parseLoopConfig(json: string): LoopConfig | null {
	if (!json || json.trim() === '' || json === 'null') {
		return null;
	}
	try {
		return JSON.parse(json) as LoopConfig;
	} catch {
		return null;
	}
}

function serializeLoopConfig(
	loopToPhase: string,
	condition: string,
	maxLoops: number,
): string {
	if (!loopToPhase) {
		return '';
	}

	const config: Record<string, unknown> = {
		loop_to_phase: loopToPhase,
		max_loops: maxLoops,
	};

	// Parse condition and include if valid
	if (condition && condition.trim()) {
		try {
			config.condition = JSON.parse(condition);
		} catch {
			// Invalid JSON, skip condition
		}
	}

	return JSON.stringify(config);
}

function getEffectiveMaxLoops(config: LoopConfig | null): number {
	if (!config) return 3;
	if (config.max_loops && config.max_loops > 0) return config.max_loops;
	if (config.max_iterations && config.max_iterations > 0) return config.max_iterations;
	return 3;
}

// ─── Component ───────────────────────────────────────────────────────────────

export function LoopEditor({
	loopConfig,
	onChange,
	priorPhases,
	disabled = false,
}: LoopEditorProps) {
	const parsed = useMemo(() => parseLoopConfig(loopConfig), [loopConfig]);

	const [loopToPhase, setLoopToPhase] = useState<string>(parsed?.loop_to_phase ?? '');
	const [condition, setCondition] = useState<string>(
		parsed?.condition ? JSON.stringify(parsed.condition) : '',
	);
	const [maxLoops, setMaxLoops] = useState<number>(getEffectiveMaxLoops(parsed));

	// Reset state when loopConfig prop changes
	useEffect(() => {
		const cfg = parseLoopConfig(loopConfig);
		setLoopToPhase(cfg?.loop_to_phase ?? '');
		setCondition(cfg?.condition ? JSON.stringify(cfg.condition) : '');
		setMaxLoops(getEffectiveMaxLoops(cfg));
	}, [loopConfig]);

	// Emit changes
	const emitChange = useCallback(
		(newLoopToPhase: string, newCondition: string, newMaxLoops: number) => {
			const json = serializeLoopConfig(newLoopToPhase, newCondition, newMaxLoops);
			onChange(json);
		},
		[onChange],
	);

	const handleLoopToPhaseChange = useCallback(
		(value: string) => {
			setLoopToPhase(value);
			emitChange(value, condition, maxLoops);
		},
		[condition, maxLoops, emitChange],
	);

	const handleConditionChange = useCallback(
		(value: string) => {
			setCondition(value);
			emitChange(loopToPhase, value, maxLoops);
		},
		[loopToPhase, maxLoops, emitChange],
	);

	const handleMaxLoopsChange = useCallback(
		(value: number) => {
			const clamped = Math.max(1, Math.min(10, value));
			setMaxLoops(clamped);
			emitChange(loopToPhase, condition, clamped);
		},
		[loopToPhase, condition, emitChange],
	);

	const handleClearLoop = useCallback(() => {
		setLoopToPhase('');
		setCondition('');
		setMaxLoops(3);
		onChange('');
	}, [onChange]);

	const hasLoop = loopToPhase !== '';

	return (
		<div className="loop-editor">
			{/* Loop target dropdown */}
			<div className="loop-editor__field">
				<label className="loop-editor__label" htmlFor="loop-to-phase">
					Loop back to
				</label>
				<div className="loop-editor__select-wrapper">
					<select
						id="loop-to-phase"
						className="loop-editor__select"
						value={loopToPhase}
						onChange={(e) => handleLoopToPhaseChange(e.target.value)}
						disabled={disabled}
						aria-label="Loop back to phase"
					>
						<option value="">No loop</option>
						{priorPhases.map((phaseId) => (
							<option key={phaseId} value={phaseId}>
								{phaseId}
							</option>
						))}
					</select>
					{hasLoop && !disabled && (
						<button
							type="button"
							className="loop-editor__clear-btn"
							onClick={handleClearLoop}
							aria-label="Clear loop configuration"
						>
							Clear
						</button>
					)}
				</div>
				<p className="loop-editor__hint">
					{priorPhases.length === 0
						? 'No prior phases available for looping'
						: 'Select a prior phase to loop back to when condition is met'}
				</p>
			</div>

			{/* Only show condition and max loops when a target is selected */}
			{hasLoop && (
				<>
					{/* Condition */}
					<div className="loop-editor__field">
						<label className="loop-editor__label">When</label>
						<p className="loop-editor__hint loop-editor__hint--above">
							Define when to loop back (e.g., when review status indicates changes needed)
						</p>
						<ConditionEditor
							condition={condition}
							onChange={handleConditionChange}
							disabled={disabled}
						/>
					</div>

					{/* Max loops */}
					<div className="loop-editor__field">
						<label className="loop-editor__label" htmlFor="max-loops">
							Max loops
						</label>
						<input
							id="max-loops"
							type="number"
							className="loop-editor__input"
							value={maxLoops}
							onChange={(e) => handleMaxLoopsChange(parseInt(e.target.value, 10) || 1)}
							min={1}
							max={10}
							disabled={disabled}
							aria-label="Maximum loop iterations"
						/>
						<p className="loop-editor__hint">
							Maximum number of loop iterations before continuing forward (1-10)
						</p>
					</div>
				</>
			)}
		</div>
	);
}

export type { LoopConfig };
