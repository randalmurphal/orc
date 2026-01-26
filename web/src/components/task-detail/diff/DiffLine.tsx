import type { DiffLine as Line } from '@/gen/orc/v1/common_pb';
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
			<span className="line-number old">{line.oldLine ?? ''}</span>
			<span className="line-number new">{line.newLine ?? ''}</span>
			<span className="line-content">
				<span className="prefix">{getPrefix()}</span>
				{line.content}
			</span>
		</div>
	);
}
