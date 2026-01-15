/**
 * Radix UI Validation Tests
 *
 * These tests verify that Radix UI primitives work correctly with React 19.
 * They serve as a validation of the Radix UI installation and compatibility.
 *
 * Note: These are basic render tests. Complex interactions (hover, keyboard nav)
 * are better tested via E2E tests in a real browser environment as jsdom has
 * limitations with ResizeObserver, scrollIntoView, and pointer events.
 */

import { describe, it, expect, vi, beforeAll } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import * as Dialog from '@radix-ui/react-dialog';
import * as DropdownMenu from '@radix-ui/react-dropdown-menu';
import * as Select from '@radix-ui/react-select';
import * as Tooltip from '@radix-ui/react-tooltip';
import * as Tabs from '@radix-ui/react-tabs';
import * as Popover from '@radix-ui/react-popover';
import { Slot } from '@radix-ui/react-slot';

// Mock browser APIs not available in jsdom
beforeAll(() => {
	Element.prototype.scrollIntoView = vi.fn();
	global.ResizeObserver = vi.fn().mockImplementation(() => ({
		observe: vi.fn(),
		unobserve: vi.fn(),
		disconnect: vi.fn(),
	}));
});

describe('Radix UI React 19 Compatibility', () => {
	describe('Package imports', () => {
		it('imports all packages without errors', () => {
			// If these imports fail, the test suite won't even run
			expect(Dialog.Root).toBeDefined();
			expect(DropdownMenu.Root).toBeDefined();
			expect(Select.Root).toBeDefined();
			expect(Tooltip.Root).toBeDefined();
			expect(Tabs.Root).toBeDefined();
			expect(Popover.Root).toBeDefined();
			expect(Slot).toBeDefined();
		});
	});

	describe('Dialog', () => {
		it('renders and can be opened/closed', async () => {
			const onOpenChange = vi.fn();

			render(
				<Dialog.Root onOpenChange={onOpenChange}>
					<Dialog.Trigger asChild>
						<button>Open Dialog</button>
					</Dialog.Trigger>
					<Dialog.Portal>
						<Dialog.Overlay />
						<Dialog.Content aria-describedby={undefined}>
							<Dialog.Title>Test Dialog</Dialog.Title>
							<Dialog.Description>This is a test dialog</Dialog.Description>
							<Dialog.Close asChild>
								<button>Close</button>
							</Dialog.Close>
						</Dialog.Content>
					</Dialog.Portal>
				</Dialog.Root>
			);

			// Trigger should be rendered
			expect(screen.getByRole('button', { name: 'Open Dialog' })).toBeInTheDocument();

			// Open the dialog
			fireEvent.click(screen.getByRole('button', { name: 'Open Dialog' }));
			expect(onOpenChange).toHaveBeenCalledWith(true);

			// Dialog content should be visible
			await waitFor(() => {
				expect(screen.getByRole('dialog')).toBeInTheDocument();
			});
			expect(screen.getByText('Test Dialog')).toBeInTheDocument();
			expect(screen.getByText('This is a test dialog')).toBeInTheDocument();

			// Close the dialog
			fireEvent.click(screen.getByRole('button', { name: 'Close' }));
			expect(onOpenChange).toHaveBeenCalledWith(false);
		});

		it('supports controlled mode', () => {
			const { rerender } = render(
				<Dialog.Root open={false}>
					<Dialog.Portal>
						<Dialog.Content aria-describedby={undefined}>
							<Dialog.Title>Controlled Dialog</Dialog.Title>
						</Dialog.Content>
					</Dialog.Portal>
				</Dialog.Root>
			);

			// Dialog should not be visible when closed
			expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

			// Rerender with open=true
			rerender(
				<Dialog.Root open={true}>
					<Dialog.Portal>
						<Dialog.Content aria-describedby={undefined}>
							<Dialog.Title>Controlled Dialog</Dialog.Title>
						</Dialog.Content>
					</Dialog.Portal>
				</Dialog.Root>
			);

			// Dialog should be visible when open
			expect(screen.getByRole('dialog')).toBeInTheDocument();
		});
	});

	describe('DropdownMenu', () => {
		it('renders trigger correctly', () => {
			render(
				<DropdownMenu.Root>
					<DropdownMenu.Trigger asChild>
						<button>Open Menu</button>
					</DropdownMenu.Trigger>
					<DropdownMenu.Portal>
						<DropdownMenu.Content>
							<DropdownMenu.Item>Item 1</DropdownMenu.Item>
							<DropdownMenu.Item>Item 2</DropdownMenu.Item>
						</DropdownMenu.Content>
					</DropdownMenu.Portal>
				</DropdownMenu.Root>
			);

			// Trigger should be rendered with correct ARIA attributes
			const trigger = screen.getByRole('button', { name: 'Open Menu' });
			expect(trigger).toBeInTheDocument();
			expect(trigger).toHaveAttribute('aria-haspopup', 'menu');
			expect(trigger).toHaveAttribute('aria-expanded', 'false');
		});

		it('supports defaultOpen for testing content', () => {
			render(
				<DropdownMenu.Root defaultOpen>
					<DropdownMenu.Trigger asChild>
						<button>Open Menu</button>
					</DropdownMenu.Trigger>
					<DropdownMenu.Portal>
						<DropdownMenu.Content>
							<DropdownMenu.Item>Item 1</DropdownMenu.Item>
							<DropdownMenu.Item>Item 2</DropdownMenu.Item>
						</DropdownMenu.Content>
					</DropdownMenu.Portal>
				</DropdownMenu.Root>
			);

			// With defaultOpen, menu should be visible
			expect(screen.getByRole('menu')).toBeInTheDocument();
			expect(screen.getByRole('menuitem', { name: 'Item 1' })).toBeInTheDocument();
			expect(screen.getByRole('menuitem', { name: 'Item 2' })).toBeInTheDocument();
		});
	});

	describe('Select', () => {
		it('renders trigger correctly', () => {
			render(
				<Select.Root>
					<Select.Trigger aria-label="Food">
						<Select.Value placeholder="Select a fruit" />
						<Select.Icon />
					</Select.Trigger>
					<Select.Portal>
						<Select.Content position="popper">
							<Select.Viewport>
								<Select.Item value="apple">
									<Select.ItemText>Apple</Select.ItemText>
								</Select.Item>
							</Select.Viewport>
						</Select.Content>
					</Select.Portal>
				</Select.Root>
			);

			// Trigger should be rendered
			expect(screen.getByRole('combobox')).toBeInTheDocument();
			expect(screen.getByText('Select a fruit')).toBeInTheDocument();
		});

		it('handles value changes', async () => {
			const onValueChange = vi.fn();

			render(
				<Select.Root onValueChange={onValueChange}>
					<Select.Trigger aria-label="Food">
						<Select.Value placeholder="Select a fruit" />
						<Select.Icon />
					</Select.Trigger>
					<Select.Portal>
						<Select.Content position="popper">
							<Select.Viewport>
								<Select.Item value="apple">
									<Select.ItemText>Apple</Select.ItemText>
								</Select.Item>
								<Select.Item value="banana">
									<Select.ItemText>Banana</Select.ItemText>
								</Select.Item>
							</Select.Viewport>
						</Select.Content>
					</Select.Portal>
				</Select.Root>
			);

			// Open the select
			fireEvent.click(screen.getByRole('combobox'));

			// Options should be visible
			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});

			// Select an option
			fireEvent.click(screen.getByRole('option', { name: 'Banana' }));
			expect(onValueChange).toHaveBeenCalledWith('banana');
		});
	});

	describe('Tooltip', () => {
		it('renders with defaultOpen', async () => {
			render(
				<Tooltip.Provider delayDuration={0}>
					<Tooltip.Root defaultOpen>
						<Tooltip.Trigger asChild>
							<button>Hover me</button>
						</Tooltip.Trigger>
						<Tooltip.Portal>
							<Tooltip.Content sideOffset={5}>
								<span>Tooltip content</span>
								<Tooltip.Arrow />
							</Tooltip.Content>
						</Tooltip.Portal>
					</Tooltip.Root>
				</Tooltip.Provider>
			);

			// Trigger should be rendered
			expect(screen.getByRole('button', { name: 'Hover me' })).toBeInTheDocument();

			// Tooltip should be visible with defaultOpen
			await waitFor(() => {
				expect(screen.getByRole('tooltip')).toBeInTheDocument();
			});
			// Radix duplicates content for accessibility (visual + screen reader)
			// Using getAllByText since content appears in multiple places
			expect(screen.getAllByText('Tooltip content').length).toBeGreaterThan(0);
		});
	});

	describe('Tabs', () => {
		it('renders tabs structure correctly', () => {
			render(
				<Tabs.Root defaultValue="tab1">
					<Tabs.List>
						<Tabs.Trigger value="tab1">Tab 1</Tabs.Trigger>
						<Tabs.Trigger value="tab2">Tab 2</Tabs.Trigger>
						<Tabs.Trigger value="tab3">Tab 3</Tabs.Trigger>
					</Tabs.List>
					<Tabs.Content value="tab1">Content 1</Tabs.Content>
					<Tabs.Content value="tab2">Content 2</Tabs.Content>
					<Tabs.Content value="tab3">Content 3</Tabs.Content>
				</Tabs.Root>
			);

			// Tab list and triggers should be rendered
			expect(screen.getByRole('tablist')).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: 'Tab 1' })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: 'Tab 2' })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: 'Tab 3' })).toBeInTheDocument();

			// First tab should be active by default
			expect(screen.getByRole('tab', { name: 'Tab 1' })).toHaveAttribute('data-state', 'active');
			expect(screen.getByRole('tab', { name: 'Tab 2' })).toHaveAttribute('data-state', 'inactive');

			// First tab content should be visible
			expect(screen.getByRole('tabpanel')).toBeInTheDocument();
			expect(screen.getByText('Content 1')).toBeInTheDocument();
		});

		it('responds to controlled value', () => {
			const { rerender } = render(
				<Tabs.Root value="tab1">
					<Tabs.List>
						<Tabs.Trigger value="tab1">Tab 1</Tabs.Trigger>
						<Tabs.Trigger value="tab2">Tab 2</Tabs.Trigger>
					</Tabs.List>
					<Tabs.Content value="tab1">Content 1</Tabs.Content>
					<Tabs.Content value="tab2">Content 2</Tabs.Content>
				</Tabs.Root>
			);

			// Tab 1 should be active
			expect(screen.getByRole('tab', { name: 'Tab 1' })).toHaveAttribute('data-state', 'active');
			expect(screen.getByText('Content 1')).toBeInTheDocument();

			// Switch to tab 2 via controlled value
			rerender(
				<Tabs.Root value="tab2">
					<Tabs.List>
						<Tabs.Trigger value="tab1">Tab 1</Tabs.Trigger>
						<Tabs.Trigger value="tab2">Tab 2</Tabs.Trigger>
					</Tabs.List>
					<Tabs.Content value="tab1">Content 1</Tabs.Content>
					<Tabs.Content value="tab2">Content 2</Tabs.Content>
				</Tabs.Root>
			);

			// Tab 2 should now be active
			expect(screen.getByRole('tab', { name: 'Tab 2' })).toHaveAttribute('data-state', 'active');
			expect(screen.getByText('Content 2')).toBeInTheDocument();
		});
	});

	describe('Popover', () => {
		it('renders and can be opened', async () => {
			const onOpenChange = vi.fn();

			render(
				<Popover.Root onOpenChange={onOpenChange}>
					<Popover.Trigger asChild>
						<button>Open Popover</button>
					</Popover.Trigger>
					<Popover.Portal>
						<Popover.Content sideOffset={5}>
							<div>Popover content here</div>
							<Popover.Arrow />
							<Popover.Close asChild>
								<button aria-label="Close">X</button>
							</Popover.Close>
						</Popover.Content>
					</Popover.Portal>
				</Popover.Root>
			);

			// Trigger should be rendered
			expect(screen.getByRole('button', { name: 'Open Popover' })).toBeInTheDocument();

			// Open the popover
			await act(async () => {
				fireEvent.click(screen.getByRole('button', { name: 'Open Popover' }));
			});
			await waitFor(() => {
				expect(onOpenChange).toHaveBeenCalledWith(true);
			});

			// Popover content should be visible
			await waitFor(() => {
				expect(screen.getByText('Popover content here')).toBeInTheDocument();
			});
		});
	});

	describe('Slot', () => {
		it('merges props onto child element', () => {
			render(
				<Slot className="slot-class" data-testid="slot-wrapper">
					<button className="button-class">Click me</button>
				</Slot>
			);

			const button = screen.getByRole('button', { name: 'Click me' });
			expect(button).toHaveClass('slot-class');
			expect(button).toHaveClass('button-class');
			expect(button).toHaveAttribute('data-testid', 'slot-wrapper');
		});
	});
});
