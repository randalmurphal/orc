/**
 * Minimal REST API functions for features not yet migrated to Connect RPC.
 *
 * These are kept separate from the Connect clients in client.ts.
 * TODO: Migrate these to proto services when notification and MCP protos are added.
 */

const API_BASE = '/api';

async function fetchJSON<T>(path: string, options?: RequestInit): Promise<T> {
	const res = await fetch(`${API_BASE}${path}`, {
		...options,
		headers: {
			'Content-Type': 'application/json',
			...options?.headers,
		},
	});
	if (!res.ok) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
	return res.json();
}

// ============================================================================
// MCP Servers (.mcp.json)
// ============================================================================

export interface MCPServerInfo {
	name: string;
	type: string;
	command?: string;
	url?: string;
	disabled: boolean;
	has_env: boolean;
	env_count: number;
	args_count: number;
}

export interface MCPServer {
	name: string;
	type: string;
	command?: string;
	args?: string[];
	env?: Record<string, string>;
	url?: string;
	headers?: string[];
	disabled: boolean;
}

export interface MCPServerCreate {
	name: string;
	type?: string;
	command?: string;
	args?: string[];
	env?: Record<string, string>;
	url?: string;
	headers?: string[];
	disabled?: boolean;
}

export async function listMCPServers(scope?: 'global'): Promise<MCPServerInfo[]> {
	const params = scope ? `?scope=${scope}` : '';
	return fetchJSON<MCPServerInfo[]>(`/mcp${params}`);
}

export async function getMCPServer(name: string): Promise<MCPServer> {
	return fetchJSON<MCPServer>(`/mcp/${name}`);
}

export async function createMCPServer(server: MCPServerCreate): Promise<MCPServerInfo> {
	return fetchJSON<MCPServerInfo>('/mcp', {
		method: 'POST',
		body: JSON.stringify(server),
	});
}

export async function updateMCPServer(
	name: string,
	server: Partial<MCPServerCreate>
): Promise<MCPServerInfo> {
	return fetchJSON<MCPServerInfo>(`/mcp/${name}`, {
		method: 'PUT',
		body: JSON.stringify(server),
	});
}

export async function deleteMCPServer(name: string): Promise<void> {
	const res = await fetch(`${API_BASE}/mcp/${name}`, { method: 'DELETE' });
	if (!res.ok && res.status !== 204) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}
}

// ============================================================================
// Task Export (uses REST for server-side file saving)
// ============================================================================

export interface ExportRequest {
	task_definition?: boolean;
	final_state?: boolean;
	transcripts?: boolean;
	context_summary?: boolean;
	to_branch?: boolean;
}

export interface ExportResponse {
	success: boolean;
	task_id: string;
	exported_to: string;
	files?: string[];
	committed_sha?: string;
}

export async function exportTask(taskId: string, options: ExportRequest): Promise<ExportResponse> {
	return fetchJSON<ExportResponse>(`/tasks/${taskId}/export`, {
		method: 'POST',
		body: JSON.stringify(options),
	});
}

// ============================================================================
// Notifications
// ============================================================================

export interface Notification {
	id: string;
	type: string;
	title: string;
	message?: string;
	source_type?: string;
	source_id?: string;
	created_at: string;
	expires_at?: string;
}

export async function listNotifications(): Promise<Notification[]> {
	const data = await fetchJSON<{ notifications: Notification[] }>('/notifications');
	return data.notifications || [];
}

export async function dismissNotification(id: string): Promise<void> {
	await fetchJSON(`/notifications/${id}/dismiss`, { method: 'POST' });
}

export async function dismissAllNotifications(): Promise<void> {
	await fetchJSON('/notifications/dismiss-all', { method: 'POST' });
}
