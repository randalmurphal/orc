import { create } from '@bufbuild/protobuf';
import { useEffect, useState } from 'react';
import type { Agent, Hook, Skill } from '@/gen/orc/v1/config_pb';
import { GetMCPServerRequestSchema, type MCPServerInfo } from '@/gen/orc/v1/mcp_pb';
import { configClient, mcpClient } from '@/lib/client';

export interface PhaseTemplateLibrariesState {
	agents: Agent[];
	agentsLoading: boolean;
	hooks: Hook[];
	hooksError: string;
	hooksLoading: boolean;
	skills: Skill[];
	skillsError: string;
	skillsLoading: boolean;
	mcpServers: MCPServerInfo[];
	mcpError: string;
	mcpLoading: boolean;
}

export function usePhaseTemplateLibraries(): PhaseTemplateLibrariesState {
	const [agents, setAgents] = useState<Agent[]>([]);
	const [agentsLoading, setAgentsLoading] = useState(true);
	const [hooks, setHooks] = useState<Hook[]>([]);
	const [hooksError, setHooksError] = useState('');
	const [hooksLoading, setHooksLoading] = useState(true);
	const [skills, setSkills] = useState<Skill[]>([]);
	const [skillsError, setSkillsError] = useState('');
	const [skillsLoading, setSkillsLoading] = useState(true);
	const [mcpServers, setMcpServers] = useState<MCPServerInfo[]>([]);
	const [mcpError, setMcpError] = useState('');
	const [mcpLoading, setMcpLoading] = useState(true);

	useEffect(() => {
		let mounted = true;

		configClient.listAgents({}).then((response) => {
			if (mounted) {
				setAgents(response.agents);
				setAgentsLoading(false);
			}
		}).catch(() => {
			if (mounted) {
				setAgentsLoading(false);
			}
		});

		configClient.listHooks({}).then((response) => {
			if (mounted) {
				setHooks(response.hooks);
				setHooksLoading(false);
			}
		}).catch(() => {
			if (mounted) {
				setHooksError('Failed to load hooks');
				setHooksLoading(false);
			}
		});

		configClient.listSkills({}).then((response) => {
			if (mounted) {
				setSkills(response.skills);
				setSkillsLoading(false);
			}
		}).catch(() => {
			if (mounted) {
				setSkillsError('Failed to load skills');
				setSkillsLoading(false);
			}
		});

		mcpClient.listMCPServers({}).then((response) => {
			if (mounted) {
				setMcpServers(response.servers);
				setMcpLoading(false);
			}
		}).catch(() => {
			if (mounted) {
				setMcpError('Failed to load MCP servers');
				setMcpLoading(false);
			}
		});

		return () => {
			mounted = false;
		};
	}, []);

	return {
		agents,
		agentsLoading,
		hooks,
		hooksError,
		hooksLoading,
		skills,
		skillsError,
		skillsLoading,
		mcpServers,
		mcpError,
		mcpLoading,
	};
}

export async function fetchMCPServerDefinition(name: string) {
	const response = await mcpClient.getMCPServer(create(GetMCPServerRequestSchema, { name }));
	if (!response.server) {
		return undefined;
	}
	return {
		type: response.server.type,
		command: response.server.command,
		args: response.server.args,
		env: response.server.env,
		url: response.server.url,
		headers: response.server.headers,
		disabled: response.server.disabled,
	};
}
