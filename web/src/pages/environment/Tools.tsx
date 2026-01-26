/**
 * Tools page (/environment/tools)
 * Displays available Claude Code tools by category
 */

import { useState, useEffect, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { useDocumentTitle } from '@/hooks';
import { configClient } from '@/lib/client';
import {
	type ToolInfo,
	type ToolPermissions,
	type ToolList,
	ListToolsRequestSchema,
	GetToolPermissionsRequestSchema,
} from '@/gen/orc/v1/config_pb';
import './environment.css';

// Convert protobuf byCategory map to a more usable format
type ToolsByCategory = { [category: string]: ToolInfo[] };

export function Tools() {
	useDocumentTitle('Tools');
	const [tools, setTools] = useState<ToolsByCategory>({});
	const [permissions, setPermissions] = useState<ToolPermissions | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	const loadTools = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const [toolsResponse, permsResponse] = await Promise.all([
				configClient.listTools(create(ListToolsRequestSchema, { byCategory: true })),
				configClient.getToolPermissions(create(GetToolPermissionsRequestSchema, {})).catch(() => null),
			]);

			// Convert protobuf ToolList map to ToolInfo[] map
			const byCategory: ToolsByCategory = {};
			for (const [category, toolList] of Object.entries(toolsResponse.byCategory)) {
				byCategory[category] = (toolList as ToolList).tools;
			}
			setTools(byCategory);
			setPermissions(permsResponse?.permissions ?? null);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load tools');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadTools();
	}, [loadTools]);

	const isAllowed = (toolName: string): boolean | null => {
		if (!permissions) return null;
		if (permissions.deny?.includes(toolName)) return false;
		if (permissions.allow?.includes(toolName)) return true;
		return null;
	};

	const categories = Object.keys(tools).sort();
	const totalTools = Object.values(tools).reduce((sum, arr) => sum + arr.length, 0);

	if (loading) {
		return (
			<div className="page environment-tools-page">
				<div className="env-loading">Loading tools...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="page environment-tools-page">
				<div className="env-error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadTools}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	return (
		<div className="page environment-tools-page">
			<div className="env-page-header">
				<div>
					<h3>Tools</h3>
					<p className="env-page-description">
						{totalTools} available Claude Code tools across {categories.length} categories.
					</p>
				</div>
			</div>

			{categories.length === 0 ? (
				<div className="env-empty">
					<Icon name="tools" size={48} />
					<p>No tools available</p>
				</div>
			) : (
				<div className="tools-categories">
					{categories.map((category) => (
						<div key={category} className="tools-category-group">
							<h4 className="tools-category-title">
								{category}
								<span className="tools-category-count">{tools[category].length}</span>
							</h4>
							<table className="tools-table">
								<thead>
									<tr>
										<th>Tool</th>
										<th>Description</th>
										<th>Status</th>
									</tr>
								</thead>
								<tbody>
									{tools[category].map((tool) => {
										const allowed = isAllowed(tool.name);
										return (
											<tr key={tool.name}>
												<td className="tool-name">{tool.name}</td>
												<td className="tool-description">{tool.description}</td>
												<td className="tool-status">
													{allowed === false ? (
														<span className="tool-badge denied">
															<Icon name="x-circle" size={12} />
															Denied
														</span>
													) : allowed === true ? (
														<span className="tool-badge allowed">
															<Icon name="check-circle" size={12} />
															Allowed
														</span>
													) : (
														<span className="tool-badge default">Default</span>
													)}
												</td>
											</tr>
										);
									})}
								</tbody>
							</table>
						</div>
					))}
				</div>
			)}
		</div>
	);
}
