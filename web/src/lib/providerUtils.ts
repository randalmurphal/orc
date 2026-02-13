/** Known LLM providers */
export const PROVIDERS = [
  { value: 'claude', label: 'Claude' },
  { value: 'codex', label: 'Codex (OpenAI)' },
  { value: 'ollama', label: 'Ollama' },
  { value: 'lmstudio', label: 'LM Studio' },
] as const;

/** Models available per provider */
export const PROVIDER_MODELS: Record<string, { value: string; label: string }[]> = {
  claude: [
    { value: 'opus', label: 'Opus' },
    { value: 'sonnet', label: 'Sonnet' },
    { value: 'haiku', label: 'Haiku' },
  ],
  codex: [
    { value: 'gpt-5', label: 'GPT-5' },
    { value: 'gpt-4.1', label: 'GPT-4.1' },
    { value: 'o3', label: 'o3' },
  ],
  ollama: [],  // User-configured, free-form
  lmstudio: [],  // User-configured, free-form
};

/** Parse "provider:model" tuple into parts */
export function parseProviderModelTuple(tuple: string): { provider: string; model: string } {
  if (!tuple) return { provider: '', model: '' };
  const idx = tuple.indexOf(':');
  if (idx < 0) return { provider: '', model: tuple };
  return { provider: tuple.slice(0, idx), model: tuple.slice(idx + 1) };
}

/** Format provider:model back to tuple */
export function formatProviderModelTuple(provider: string, model: string): string {
  if (!provider || provider === 'claude') return model;
  if (!model) return '';
  return `${provider}:${model}`;
}

/** Get display label for a provider */
export function providerLabel(provider: string): string {
  if (!provider) return 'Default';
  const found = PROVIDERS.find(p => p.value === provider);
  return found?.label ?? provider;
}
