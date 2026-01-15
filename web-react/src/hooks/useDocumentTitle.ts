import { useEffect } from 'react';

/**
 * Set the document title, following the orc convention.
 * 
 * Examples:
 * - useDocumentTitle('Tasks') → 'orc - Tasks'
 * - useDocumentTitle(task?.title) → 'Task Title - orc' or 'orc' if no title
 */
export function useDocumentTitle(title?: string | null) {
	useEffect(() => {
		if (title) {
			document.title = title.includes('orc') ? title : `orc - ${title}`;
		} else {
			document.title = 'orc';
		}
	}, [title]);
}
