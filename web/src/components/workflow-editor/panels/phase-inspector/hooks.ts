import { useEffect, useState } from 'react';
import { configClient, mcpClient } from '@/lib/client';
import type { Agent, Hook, Skill } from '@/gen/orc/v1/config_pb';
import type { MCPServerInfo } from '@/gen/orc/v1/mcp_pb';

export function useMobileViewport() {
	const [isMobile, setIsMobile] = useState(false);

	useEffect(() => {
		if (typeof window === 'undefined' || !window.matchMedia) {
			return;
		}

		const mediaQuery = window.matchMedia('(max-width: 640px)');
		setIsMobile(mediaQuery.matches);

		const handleChange = (e: MediaQueryListEvent) => {
			setIsMobile(e.matches);
		};

		mediaQuery.addEventListener('change', handleChange);
		return () => mediaQuery.removeEventListener('change', handleChange);
	}, []);

	return isMobile;
}

export interface PhaseInspectorLibraryData {
	agents: Agent[];
	hooks: Hook[];
	skills: Skill[];
	mcpServers: MCPServerInfo[];
	agentsLoading: boolean;
	hooksLoading: boolean;
	skillsLoading: boolean;
	mcpLoading: boolean;
}

export function usePhaseInspectorLibraryData(): PhaseInspectorLibraryData {
	const [agents, setAgents] = useState<Agent[]>([]);
	const [hooks, setHooks] = useState<Hook[]>([]);
	const [skills, setSkills] = useState<Skill[]>([]);
	const [mcpServers, setMcpServers] = useState<MCPServerInfo[]>([]);
	const [agentsLoading, setAgentsLoading] = useState(true);
	const [hooksLoading, setHooksLoading] = useState(true);
	const [skillsLoading, setSkillsLoading] = useState(true);
	const [mcpLoading, setMcpLoading] = useState(true);

	useEffect(() => {
		const loadData = async () => {
			try {
				const [agentsResp, hooksResp, skillsResp, mcpResp] = await Promise.allSettled([
					configClient.listAgents({}),
					configClient.listHooks({}),
					configClient.listSkills({}),
					mcpClient.listMCPServers({}),
				]);

				if (agentsResp.status === 'fulfilled') {
					setAgents(agentsResp.value.agents);
				}
				if (hooksResp.status === 'fulfilled') {
					setHooks(hooksResp.value.hooks);
				}
				if (skillsResp.status === 'fulfilled') {
					setSkills(skillsResp.value.skills);
				}
				if (mcpResp.status === 'fulfilled') {
					setMcpServers(mcpResp.value.servers);
				}
			} catch (error) {
				console.error('Failed to load inspector data:', error);
			} finally {
				setAgentsLoading(false);
				setHooksLoading(false);
				setSkillsLoading(false);
				setMcpLoading(false);
			}
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
	};
}
