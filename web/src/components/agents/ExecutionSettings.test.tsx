import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import { ExecutionSettings, type ExecutionSettingsData } from './ExecutionSettings';

const defaultSettings: ExecutionSettingsData = {
	parallelTasks: 2,
	autoApprove: true,
	defaultModel: 'claude-sonnet-4-20250514',
	costLimit: 25,
};

describe('ExecutionSettings', () => {
	describe('rendering', () => {
		it('renders all four setting cards', () => {
			render(<ExecutionSettings settings={defaultSettings} onChange={vi.fn()} />);

			expect(screen.getByText('Parallel Tasks')).toBeInTheDocument();
			expect(screen.getByText('Auto-Approve')).toBeInTheDocument();
			expect(screen.getByText('Default Model')).toBeInTheDocument();
			expect(screen.getByText('Cost Limit')).toBeInTheDocument();
		});

		it('renders descriptions for each setting', () => {
			render(<ExecutionSettings settings={defaultSettings} onChange={vi.fn()} />);

			expect(
				screen.getByText('Maximum number of tasks to run simultaneously')
			).toBeInTheDocument();
			expect(
				screen.getByText('Automatically approve safe operations without prompting')
			).toBeInTheDocument();
			expect(screen.getByText('Model to use for new tasks')).toBeInTheDocument();
			expect(screen.getByText('Daily spending limit before pause')).toBeInTheDocument();
		});

		it('applies settings-grid class to container', () => {
			const { container } = render(
				<ExecutionSettings settings={defaultSettings} onChange={vi.fn()} />
			);
			expect(container.querySelector('.settings-grid')).toBeInTheDocument();
		});
	});

	describe('Parallel Tasks slider', () => {
		it('shows current value', () => {
			render(<ExecutionSettings settings={defaultSettings} onChange={vi.fn()} />);
			// The slider displays the value
			expect(screen.getByText('2')).toBeInTheDocument();
		});

		it('calls onChange with correct key when value changes', () => {
			const handleChange = vi.fn();
			const { container } = render(
				<ExecutionSettings settings={defaultSettings} onChange={handleChange} />
			);

			// Find the first slider (Parallel Tasks)
			const sliders = container.querySelectorAll('.slider__track');
			expect(sliders.length).toBeGreaterThan(0);

			// Simulate mouse interaction on the slider
			const slider = sliders[0];
			const rect = { left: 0, width: 100 } as DOMRect;
			vi.spyOn(slider, 'getBoundingClientRect').mockReturnValue(rect);

			// Click at 75% (which would be value 4 for min=1, max=4)
			fireEvent.mouseDown(slider, { clientX: 75 });

			expect(handleChange).toHaveBeenCalledWith({ parallelTasks: expect.any(Number) });
		});
	});

	describe('Auto-Approve toggle', () => {
		it('calls onChange with boolean value when toggled', () => {
			const handleChange = vi.fn();
			render(
				<ExecutionSettings
					settings={{ ...defaultSettings, autoApprove: false }}
					onChange={handleChange}
				/>
			);

			const toggle = screen.getByRole('switch', { name: /auto-approve/i });
			fireEvent.click(toggle);

			expect(handleChange).toHaveBeenCalledWith({ autoApprove: true });
		});

		it('reflects current autoApprove value', () => {
			render(<ExecutionSettings settings={defaultSettings} onChange={vi.fn()} />);

			const toggle = screen.getByRole('switch', { name: /auto-approve/i });
			expect(toggle).toBeChecked();
		});

		it('shows unchecked when autoApprove is false', () => {
			render(
				<ExecutionSettings
					settings={{ ...defaultSettings, autoApprove: false }}
					onChange={vi.fn()}
				/>
			);

			const toggle = screen.getByRole('switch', { name: /auto-approve/i });
			expect(toggle).not.toBeChecked();
		});
	});

	describe('Default Model select', () => {
		it('calls onChange with selected model string', () => {
			const handleChange = vi.fn();
			render(<ExecutionSettings settings={defaultSettings} onChange={handleChange} />);

			// Click to open the select dropdown
			const selectTrigger = screen.getByRole('combobox', { name: /default model/i });
			fireEvent.click(selectTrigger);

			// Select a different option
			const option = screen.getByRole('option', { name: /claude opus 4/i });
			fireEvent.click(option);

			expect(handleChange).toHaveBeenCalledWith({ defaultModel: 'claude-opus-4-20250514' });
		});

		it('displays current model selection', () => {
			render(<ExecutionSettings settings={defaultSettings} onChange={vi.fn()} />);

			expect(screen.getByText('Claude Sonnet 4')).toBeInTheDocument();
		});
	});

	describe('Cost Limit slider', () => {
		it('formats value with $ prefix', () => {
			render(<ExecutionSettings settings={defaultSettings} onChange={vi.fn()} />);

			expect(screen.getByText('$25')).toBeInTheDocument();
		});

		it('calls onChange with correct key when value changes', () => {
			const handleChange = vi.fn();
			const { container } = render(
				<ExecutionSettings settings={defaultSettings} onChange={handleChange} />
			);

			// Find the second slider (Cost Limit)
			const sliders = container.querySelectorAll('.slider__track');
			expect(sliders.length).toBe(2);

			// Simulate interaction on the cost slider
			const costSlider = sliders[1];
			const rect = { left: 0, width: 100 } as DOMRect;
			vi.spyOn(costSlider, 'getBoundingClientRect').mockReturnValue(rect);

			fireEvent.mouseDown(costSlider, { clientX: 50 });

			expect(handleChange).toHaveBeenCalledWith({ costLimit: expect.any(Number) });
		});
	});

	describe('saving state', () => {
		it('displays saving indicator when isSaving is true', () => {
			render(<ExecutionSettings settings={defaultSettings} onChange={vi.fn()} isSaving />);

			expect(screen.getByText('Saving...')).toBeInTheDocument();
		});

		it('does not display saving indicator when isSaving is false', () => {
			render(
				<ExecutionSettings settings={defaultSettings} onChange={vi.fn()} isSaving={false} />
			);

			expect(screen.queryByText('Saving...')).not.toBeInTheDocument();
		});

		it('disables Parallel Tasks slider when isSaving is true', () => {
			const { container } = render(
				<ExecutionSettings settings={defaultSettings} onChange={vi.fn()} isSaving />
			);

			// The first slider is Parallel Tasks
			const sliders = container.querySelectorAll('.slider__track');
			expect(sliders[0]).toHaveAttribute('aria-disabled', 'true');
		});

		it('disables Auto-Approve toggle when isSaving is true', () => {
			render(<ExecutionSettings settings={defaultSettings} onChange={vi.fn()} isSaving />);

			const toggle = screen.getByRole('switch', { name: /auto-approve/i });
			expect(toggle).toHaveAttribute('aria-disabled', 'true');
		});

		it('disables Default Model select when isSaving is true', () => {
			render(<ExecutionSettings settings={defaultSettings} onChange={vi.fn()} isSaving />);

			const select = screen.getByRole('combobox', { name: /default model/i });
			expect(select).toBeDisabled();
		});

		it('disables Cost Limit slider when isSaving is true', () => {
			const { container } = render(
				<ExecutionSettings settings={defaultSettings} onChange={vi.fn()} isSaving />
			);

			// The second slider is Cost Limit
			const sliders = container.querySelectorAll('.slider__track');
			expect(sliders[1]).toHaveAttribute('aria-disabled', 'true');
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLDivElement>();
			render(<ExecutionSettings ref={ref} settings={defaultSettings} onChange={vi.fn()} />);

			expect(ref.current).toBeInstanceOf(HTMLDivElement);
			expect(ref.current).toHaveClass('execution-settings');
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = render(
				<ExecutionSettings
					settings={defaultSettings}
					onChange={vi.fn()}
					className="custom-class"
				/>
			);

			const wrapper = container.querySelector('.execution-settings');
			expect(wrapper).toHaveClass('custom-class');
			expect(wrapper).toHaveClass('execution-settings');
		});
	});
});
