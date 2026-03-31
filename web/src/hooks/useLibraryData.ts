import { useEffect, useState } from 'react';
import { configClient, mcpClient } from '@/lib/client';
import type { Agent, Hook, Skill } from '@/gen/orc/v1/config_pb';
import type { MCPServerInfo } from '@/gen/orc/v1/mcp_pb';

export interface LibraryData {
	agents: Agent[];
	hooks: Hook[];
	skills: Skill[];
	mcpServers: MCPServerInfo[];
	agentsLoading: boolean;
	hooksLoading: boolean;
	skillsLoading: boolean;
	mcpLoading: boolean;
	agentsError: string;
	hooksError: string;
	skillsError: string;
	mcpError: string;
}

export function useLibraryData(): LibraryData {
	const [agents, setAgents] = useState<Agent[]>([]);
	const [hooks, setHooks] = useState<Hook[]>([]);
	const [skills, setSkills] = useState<Skill[]>([]);
	const [mcpServers, setMcpServers] = useState<MCPServerInfo[]>([]);
	const [agentsLoading, setAgentsLoading] = useState(true);
	const [hooksLoading, setHooksLoading] = useState(true);
	const [skillsLoading, setSkillsLoading] = useState(true);
	const [mcpLoading, setMcpLoading] = useState(true);
	const [agentsError, setAgentsError] = useState('');
	const [hooksError, setHooksError] = useState('');
	const [skillsError, setSkillsError] = useState('');
	const [mcpError, setMcpError] = useState('');

	useEffect(() => {
		const loadData = async () => {
			const [agentsResp, hooksResp, skillsResp, mcpResp] = await Promise.allSettled([
				configClient.listAgents({}),
				configClient.listHooks({}),
				configClient.listSkills({}),
				mcpClient.listMCPServers({}),
			]);

			if (agentsResp.status === 'fulfilled') {
				setAgents(agentsResp.value.agents);
			} else {
				setAgentsError('Failed to load agents');
			}
			if (hooksResp.status === 'fulfilled') {
				setHooks(hooksResp.value.hooks);
			} else {
				setHooksError('Failed to load hooks');
			}
			if (skillsResp.status === 'fulfilled') {
				setSkills(skillsResp.value.skills);
			} else {
				setSkillsError('Failed to load skills');
			}
			if (mcpResp.status === 'fulfilled') {
				setMcpServers(mcpResp.value.servers);
			} else {
				setMcpError('Failed to load MCP servers');
			}

			setAgentsLoading(false);
			setHooksLoading(false);
			setSkillsLoading(false);
			setMcpLoading(false);
		};

		void loadData();
	}, []);

	return {
		agents,
		hooks,
		skills,
		mcpServers,
		agentsLoading,
		hooksLoading,
		skillsLoading,
		mcpLoading,
		agentsError,
		hooksError,
		skillsError,
		mcpError,
	};
}
