import { useState } from 'react';
import { VariableSourceType } from '@/gen/orc/v1/workflow_pb';
import {
	type ApiConfig,
	type EnvConfig,
	type PhaseOutputConfig,
	type PromptFragmentConfig,
	type ScriptConfig,
	type SourceConfig,
	type StaticConfig,
} from './types';

interface SourceTypeRadioProps {
	value: VariableSourceType;
	selected: VariableSourceType;
	onChange: (type: VariableSourceType) => void;
	label: string;
}

export function SourceTypeRadio({
	value,
	selected,
	onChange,
	label,
}: SourceTypeRadioProps) {
	return (
		<label
			className={`variable-modal-source-radio ${selected === value ? 'variable-modal-source-radio--selected' : ''}`}
		>
			<input
				type="radio"
				name="source-type"
				checked={selected === value}
				onChange={() => onChange(value)}
				className="sr-only"
			/>
			<span>{label}</span>
		</label>
	);
}

interface SourceConfigFieldsProps {
	sourceType: VariableSourceType;
	config: SourceConfig;
	onChange: (config: SourceConfig) => void;
	availablePhases: string[];
}

export function SourceConfigFields({
	sourceType,
	config,
	onChange,
	availablePhases,
}: SourceConfigFieldsProps) {
	switch (sourceType) {
		case VariableSourceType.STATIC:
			return <StaticSourceForm config={config as StaticConfig} onChange={onChange} />;
		case VariableSourceType.ENV:
			return <EnvSourceForm config={config as EnvConfig} onChange={onChange} />;
		case VariableSourceType.SCRIPT:
			return <ScriptSourceForm config={config as ScriptConfig} onChange={onChange} />;
		case VariableSourceType.API:
			return <ApiSourceForm config={config as ApiConfig} onChange={onChange} />;
		case VariableSourceType.PHASE_OUTPUT:
			return (
				<PhaseOutputSourceForm
					config={config as PhaseOutputConfig}
					onChange={onChange}
					availablePhases={availablePhases}
				/>
			);
		case VariableSourceType.PROMPT_FRAGMENT:
			return (
				<PromptFragmentSourceForm
					config={config as PromptFragmentConfig}
					onChange={onChange}
				/>
			);
		default:
			return null;
	}
}

function StaticSourceForm({
	config,
	onChange,
}: {
	config: StaticConfig;
	onChange: (config: StaticConfig) => void;
}) {
	return (
		<div className="variable-modal-source-fields">
			<div className="variable-modal-field">
				<label htmlFor="static-value" className="variable-modal-label">
					Value <span className="variable-modal-required">*</span>
				</label>
				<textarea
					id="static-value"
					className="variable-modal-textarea"
					value={config.value}
					onChange={(event) => onChange({ ...config, value: event.target.value })}
					placeholder="The static value"
					rows={3}
				/>
			</div>
		</div>
	);
}

function EnvSourceForm({
	config,
	onChange,
}: {
	config: EnvConfig;
	onChange: (config: EnvConfig) => void;
}) {
	return (
		<div className="variable-modal-source-fields">
			<div className="variable-modal-field">
				<label htmlFor="env-var" className="variable-modal-label">
					Environment Variable <span className="variable-modal-required">*</span>
				</label>
				<input
					id="env-var"
					type="text"
					className="variable-modal-input"
					value={config.var}
					onChange={(event) => onChange({ ...config, var: event.target.value })}
					placeholder="MY_ENV_VAR"
				/>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="env-default" className="variable-modal-label">
					Default (if not set)
				</label>
				<input
					id="env-default"
					type="text"
					className="variable-modal-input"
					value={config.default ?? ''}
					onChange={(event) =>
						onChange({ ...config, default: event.target.value || undefined })
					}
					placeholder="fallback value"
				/>
			</div>
		</div>
	);
}

function ScriptSourceForm({
	config,
	onChange,
}: {
	config: ScriptConfig;
	onChange: (config: ScriptConfig) => void;
}) {
	const [argsText, setArgsText] = useState((config.args ?? []).join(' '));

	return (
		<div className="variable-modal-source-fields">
			<div className="variable-modal-field">
				<label htmlFor="script-path" className="variable-modal-label">
					Script Path <span className="variable-modal-required">*</span>
				</label>
				<input
					id="script-path"
					type="text"
					className="variable-modal-input"
					value={config.path}
					onChange={(event) => onChange({ ...config, path: event.target.value })}
					placeholder=".orc/scripts/fetch-token.sh"
				/>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="script-args" className="variable-modal-label">
					Arguments
				</label>
				<input
					id="script-args"
					type="text"
					className="variable-modal-input"
					value={argsText}
					onChange={(event) => {
						setArgsText(event.target.value);
						const args = event.target.value.split(/\s+/).filter(Boolean);
						onChange({ ...config, args: args.length > 0 ? args : undefined });
					}}
					placeholder="--env {{TASK_ID}}"
				/>
				<span className="variable-modal-hint">
					Space-separated. Supports {'{{VAR}}'} interpolation.
				</span>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="script-timeout" className="variable-modal-label">
					Timeout (ms)
				</label>
				<input
					id="script-timeout"
					type="number"
					className="variable-modal-input variable-modal-input--narrow"
					value={config.timeout ?? ''}
					onChange={(event) =>
						onChange({ ...config, timeout: parseInt(event.target.value, 10) || undefined })
					}
					placeholder="5000"
					min={0}
				/>
			</div>
		</div>
	);
}

function ApiSourceForm({
	config,
	onChange,
}: {
	config: ApiConfig;
	onChange: (config: ApiConfig) => void;
}) {
	const [headersText, setHeadersText] = useState(
		config.headers ? JSON.stringify(config.headers, null, 2) : '',
	);

	return (
		<div className="variable-modal-source-fields">
			<div className="variable-modal-field">
				<label htmlFor="api-url" className="variable-modal-label">
					URL <span className="variable-modal-required">*</span>
				</label>
				<input
					id="api-url"
					type="text"
					className="variable-modal-input"
					value={config.url}
					onChange={(event) => onChange({ ...config, url: event.target.value })}
					placeholder="https://api.example.com/data"
				/>
				<span className="variable-modal-hint">
					Supports {'{{VAR}}'} interpolation.
				</span>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="api-method" className="variable-modal-label">
					Method
				</label>
				<select
					id="api-method"
					className="variable-modal-select"
					value={config.method ?? 'GET'}
					onChange={(event) => onChange({ ...config, method: event.target.value })}
				>
					<option value="GET">GET</option>
					<option value="POST">POST</option>
					<option value="PUT">PUT</option>
				</select>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="api-headers" className="variable-modal-label">
					Headers (JSON)
				</label>
				<textarea
					id="api-headers"
					className="variable-modal-textarea"
					value={headersText}
					onChange={(event) => {
						setHeadersText(event.target.value);
						try {
							const headers = event.target.value.trim()
								? JSON.parse(event.target.value)
								: undefined;
							onChange({ ...config, headers });
						} catch {
							// Keep text local until JSON is valid.
						}
					}}
					placeholder='{"Authorization": "Bearer {{API_TOKEN}}"}'
					rows={3}
				/>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="api-jq" className="variable-modal-label">
					JQ Filter (JSONPath)
				</label>
				<input
					id="api-jq"
					type="text"
					className="variable-modal-input"
					value={config.jqFilter ?? ''}
					onChange={(event) =>
						onChange({ ...config, jqFilter: event.target.value || undefined })
					}
					placeholder="data.result"
				/>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="api-timeout" className="variable-modal-label">
					Timeout (ms)
				</label>
				<input
					id="api-timeout"
					type="number"
					className="variable-modal-input variable-modal-input--narrow"
					value={config.timeout ?? ''}
					onChange={(event) =>
						onChange({ ...config, timeout: parseInt(event.target.value, 10) || undefined })
					}
					placeholder="10000"
					min={0}
				/>
			</div>
		</div>
	);
}

function PhaseOutputSourceForm({
	config,
	onChange,
	availablePhases,
}: {
	config: PhaseOutputConfig;
	onChange: (config: PhaseOutputConfig) => void;
	availablePhases: string[];
}) {
	return (
		<div className="variable-modal-source-fields">
			<div className="variable-modal-field">
				<label htmlFor="phase-output-phase" className="variable-modal-label">
					Phase <span className="variable-modal-required">*</span>
				</label>
				{availablePhases.length > 0 ? (
					<select
						id="phase-output-phase"
						className="variable-modal-select"
						value={config.phase}
						onChange={(event) => onChange({ ...config, phase: event.target.value })}
					>
						<option value="">Select a phase</option>
						{availablePhases.map((phase) => (
							<option key={phase} value={phase}>
								{phase}
							</option>
						))}
					</select>
				) : (
					<input
						id="phase-output-phase"
						type="text"
						className="variable-modal-input"
						value={config.phase}
						onChange={(event) => onChange({ ...config, phase: event.target.value })}
						placeholder="spec"
					/>
				)}
			</div>
			<div className="variable-modal-field">
				<label htmlFor="phase-output-field" className="variable-modal-label">
					Field (optional)
				</label>
				<input
					id="phase-output-field"
					type="text"
					className="variable-modal-input"
					value={config.field ?? ''}
					onChange={(event) =>
						onChange({ ...config, field: event.target.value || undefined })
					}
					placeholder="content"
				/>
				<span className="variable-modal-hint">
					Extract a specific field from the phase output JSON
				</span>
			</div>
		</div>
	);
}

function PromptFragmentSourceForm({
	config,
	onChange,
}: {
	config: PromptFragmentConfig;
	onChange: (config: PromptFragmentConfig) => void;
}) {
	return (
		<div className="variable-modal-source-fields">
			<div className="variable-modal-field">
				<label htmlFor="fragment-path" className="variable-modal-label">
					Fragment Path <span className="variable-modal-required">*</span>
				</label>
				<input
					id="fragment-path"
					type="text"
					className="variable-modal-input"
					value={config.path}
					onChange={(event) => onChange({ ...config, path: event.target.value })}
					placeholder=".orc/prompts/fragments/code-style.md"
				/>
			</div>
		</div>
	);
}
