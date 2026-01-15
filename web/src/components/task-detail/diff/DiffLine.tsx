import type { Line } from '@/lib/types';
import './DiffLine.css';

interface DiffLineProps {
	line: Line;
	onClick?: () => void;
}

export function DiffLine({ line, onClick }: DiffLineProps) {
	const getPrefix = () => {
		switch (line.type) {
			case 'addition':
				return '+';
			case 'deletion':
				return '-';
			default:
				return ' ';
		}
	};

	return (
		<div className={`diff-line ${line.type}`} onClick={onClick}>
			<span className="line-number old">{line.old_line ?? ''}</span>
			<span className="line-number new">{line.new_line ?? ''}</span>
			<span className="line-content">
				<span className="prefix">{getPrefix()}</span>
				{line.content}
			</span>
		</div>
	);
}
