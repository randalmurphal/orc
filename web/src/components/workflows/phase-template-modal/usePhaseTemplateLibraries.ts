import { useLibraryData, type LibraryData } from '@/hooks/useLibraryData';

export type PhaseTemplateLibrariesState = LibraryData;

export function usePhaseTemplateLibraries(): PhaseTemplateLibrariesState {
	return useLibraryData();
}
