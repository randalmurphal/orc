/**
 * Board page (/board)
 *
 * Renders the new BoardView component which provides:
 * - Two-column layout (Queue + Running)
 * - Initiative swimlanes in queue
 * - Pipeline visualization for running tasks
 * - Right panel with Blocked, Decisions, Config, Files, Completed sections
 */

import { BoardView } from '@/components/board';
import { useDocumentTitle } from '@/hooks';

export function Board() {
	useDocumentTitle('Board');
	return <BoardView />;
}
