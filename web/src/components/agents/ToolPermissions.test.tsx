import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ToolPermissions, type ToolId } from './ToolPermissions';

const ALL_PERMISSIONS: Record<ToolId, boolean> = {
	file_read: true,
	file_write: true,
	bash_commands: true,
	web_search: true,
	git_operations: true,
	mcp_servers: true,
};

describe('ToolPermissions', () => {
	describe('rendering', () => {
		it('renders all 6 permission items', () => {
			render(<ToolPermissions permissions={ALL_PERMISSIONS} onChange={() => {}} />);

			expect(screen.getByText('File Read')).toBeInTheDocument();
			expect(screen.getByText('File Write')).toBeInTheDocument();
			expect(screen.getByText('Bash Commands')).toBeInTheDocument();
			expect(screen.getByText('Web Search')).toBeInTheDocument();
			expect(screen.getByText('Git Operations')).toBeInTheDocument();
			expect(screen.getByText('MCP Servers')).toBeInTheDocument();
		});

		it('renders permission grid with 3-column layout', () => {
			const { container } = render(
				<ToolPermissions permissions={ALL_PERMISSIONS} onChange={() => {}} />
			);

			const grid = container.querySelector('.tool-permissions__grid');
			expect(grid).toBeInTheDocument();
		});

		it('renders each permission with an icon and toggle', () => {
			render(<ToolPermissions permissions={ALL_PERMISSIONS} onChange={() => {}} />);

			// Should have 6 toggles
			const toggles = screen.getAllByRole('switch');
			expect(toggles).toHaveLength(6);
		});
	});

	describe('toggle state', () => {
		it('shows toggles as checked when permissions are enabled', () => {
			render(<ToolPermissions permissions={ALL_PERMISSIONS} onChange={() => {}} />);

			const toggles = screen.getAllByRole('switch');
			toggles.forEach((toggle) => {
				expect(toggle).toBeChecked();
			});
		});

		it('shows toggles as unchecked when permissions are disabled', () => {
			const disabledPermissions: Record<ToolId, boolean> = {
				file_read: false,
				file_write: false,
				bash_commands: false,
				web_search: false,
				git_operations: false,
				mcp_servers: false,
			};

			render(<ToolPermissions permissions={disabledPermissions} onChange={() => {}} />);

			const toggles = screen.getAllByRole('switch');
			toggles.forEach((toggle) => {
				expect(toggle).not.toBeChecked();
			});
		});

		it('reflects mixed permission states', () => {
			const mixedPermissions: Record<ToolId, boolean> = {
				file_read: true,
				file_write: false,
				bash_commands: true,
				web_search: false,
				git_operations: true,
				mcp_servers: false,
			};

			render(<ToolPermissions permissions={mixedPermissions} onChange={() => {}} />);

			const fileReadToggle = screen.getByLabelText('File Read permission');
			const fileWriteToggle = screen.getByLabelText('File Write permission');

			expect(fileReadToggle).toBeChecked();
			expect(fileWriteToggle).not.toBeChecked();
		});
	});

	describe('onChange callback', () => {
		it('calls onChange when toggling non-critical permission on', () => {
			const handleChange = vi.fn();
			const permissions = { ...ALL_PERMISSIONS, web_search: false };

			render(<ToolPermissions permissions={permissions} onChange={handleChange} />);

			const webSearchToggle = screen.getByLabelText('Web Search permission');
			fireEvent.click(webSearchToggle);

			expect(handleChange).toHaveBeenCalledWith('web_search', true);
		});

		it('calls onChange when toggling non-critical permission off', () => {
			const handleChange = vi.fn();

			render(<ToolPermissions permissions={ALL_PERMISSIONS} onChange={handleChange} />);

			const webSearchToggle = screen.getByLabelText('Web Search permission');
			fireEvent.click(webSearchToggle);

			expect(handleChange).toHaveBeenCalledWith('web_search', false);
		});

		it('calls onChange with correct tool ID for each permission', () => {
			const handleChange = vi.fn();
			const permissions = { ...ALL_PERMISSIONS, file_read: false };

			render(<ToolPermissions permissions={permissions} onChange={handleChange} />);

			const fileReadToggle = screen.getByLabelText('File Read permission');
			fireEvent.click(fileReadToggle);

			expect(handleChange).toHaveBeenCalledWith('file_read', true);
		});
	});

	describe('critical permission warnings', () => {
		it('shows warning when disabling file_write', () => {
			const handleChange = vi.fn();

			render(<ToolPermissions permissions={ALL_PERMISSIONS} onChange={handleChange} />);

			const fileWriteToggle = screen.getByLabelText('File Write permission');
			fireEvent.click(fileWriteToggle);

			expect(screen.getByText(/Disabling/)).toBeInTheDocument();
			// File Write appears both in label and warning; check the warning contains it
			expect(screen.getByText(/may limit agent capabilities/)).toBeInTheDocument();
			expect(handleChange).not.toHaveBeenCalled();
		});

		it('shows warning when disabling bash_commands', () => {
			const handleChange = vi.fn();

			render(<ToolPermissions permissions={ALL_PERMISSIONS} onChange={handleChange} />);

			const bashToggle = screen.getByLabelText('Bash Commands permission');
			fireEvent.click(bashToggle);

			expect(screen.getByText(/Disabling/)).toBeInTheDocument();
			// Check warning dialog is present
			expect(screen.getByText(/may limit agent capabilities/)).toBeInTheDocument();
		});

		it('calls onChange when confirming disable', () => {
			const handleChange = vi.fn();

			render(<ToolPermissions permissions={ALL_PERMISSIONS} onChange={handleChange} />);

			const fileWriteToggle = screen.getByLabelText('File Write permission');
			fireEvent.click(fileWriteToggle);

			const disableBtn = screen.getByRole('button', { name: /disable/i });
			fireEvent.click(disableBtn);

			expect(handleChange).toHaveBeenCalledWith('file_write', false);
		});

		it('does not call onChange when canceling disable', () => {
			const handleChange = vi.fn();

			render(<ToolPermissions permissions={ALL_PERMISSIONS} onChange={handleChange} />);

			const fileWriteToggle = screen.getByLabelText('File Write permission');
			fireEvent.click(fileWriteToggle);

			const cancelBtn = screen.getByRole('button', { name: /cancel/i });
			fireEvent.click(cancelBtn);

			expect(handleChange).not.toHaveBeenCalled();
		});

		it('closes warning after confirming', () => {
			const handleChange = vi.fn();

			render(<ToolPermissions permissions={ALL_PERMISSIONS} onChange={handleChange} />);

			const fileWriteToggle = screen.getByLabelText('File Write permission');
			fireEvent.click(fileWriteToggle);

			const disableBtn = screen.getByRole('button', { name: /disable/i });
			fireEvent.click(disableBtn);

			expect(screen.queryByText(/Disabling/)).not.toBeInTheDocument();
		});

		it('closes warning after canceling', () => {
			render(<ToolPermissions permissions={ALL_PERMISSIONS} onChange={() => {}} />);

			const bashToggle = screen.getByLabelText('Bash Commands permission');
			fireEvent.click(bashToggle);

			expect(screen.getByText(/Disabling/)).toBeInTheDocument();

			const cancelBtn = screen.getByRole('button', { name: /cancel/i });
			fireEvent.click(cancelBtn);

			expect(screen.queryByText(/Disabling/)).not.toBeInTheDocument();
		});

		it('does not show warning when showWarnings is false', () => {
			const handleChange = vi.fn();

			render(
				<ToolPermissions
					permissions={ALL_PERMISSIONS}
					onChange={handleChange}
					showWarnings={false}
				/>
			);

			const fileWriteToggle = screen.getByLabelText('File Write permission');
			fireEvent.click(fileWriteToggle);

			expect(screen.queryByText(/Disabling/)).not.toBeInTheDocument();
			expect(handleChange).toHaveBeenCalledWith('file_write', false);
		});
	});

	describe('loading state', () => {
		it('disables all toggles when loading', () => {
			render(<ToolPermissions permissions={ALL_PERMISSIONS} onChange={() => {}} loading />);

			const toggles = screen.getAllByRole('switch');
			toggles.forEach((toggle) => {
				expect(toggle).toHaveAttribute('aria-disabled', 'true');
			});
		});

		it('prevents interactions when loading', () => {
			const handleChange = vi.fn();

			render(
				<ToolPermissions permissions={ALL_PERMISSIONS} onChange={handleChange} loading />
			);

			const webSearchToggle = screen.getByLabelText('Web Search permission');
			fireEvent.click(webSearchToggle);

			expect(handleChange).not.toHaveBeenCalled();
		});
	});

	describe('accessibility', () => {
		it('has accessible labels for all toggles', () => {
			render(<ToolPermissions permissions={ALL_PERMISSIONS} onChange={() => {}} />);

			expect(screen.getByLabelText('File Read permission')).toBeInTheDocument();
			expect(screen.getByLabelText('File Write permission')).toBeInTheDocument();
			expect(screen.getByLabelText('Bash Commands permission')).toBeInTheDocument();
			expect(screen.getByLabelText('Web Search permission')).toBeInTheDocument();
			expect(screen.getByLabelText('Git Operations permission')).toBeInTheDocument();
			expect(screen.getByLabelText('MCP Servers permission')).toBeInTheDocument();
		});

		it('toggles are keyboard accessible', () => {
			render(<ToolPermissions permissions={ALL_PERMISSIONS} onChange={() => {}} />);

			const toggle = screen.getByLabelText('Web Search permission');
			toggle.focus();
			expect(document.activeElement).toBe(toggle);
		});
	});

	describe('default permissions', () => {
		it('treats missing permissions as enabled by default', () => {
			render(<ToolPermissions permissions={{}} onChange={() => {}} />);

			const toggles = screen.getAllByRole('switch');
			toggles.forEach((toggle) => {
				expect(toggle).toBeChecked();
			});
		});
	});

	describe('CSS classes', () => {
		it('applies warning class to item when warning is active', () => {
			const { container } = render(
				<ToolPermissions permissions={ALL_PERMISSIONS} onChange={() => {}} />
			);

			const fileWriteToggle = screen.getByLabelText('File Write permission');
			fireEvent.click(fileWriteToggle);

			const warningItem = container.querySelector('.tool-permissions__item--warning');
			expect(warningItem).toBeInTheDocument();
		});
	});
});
