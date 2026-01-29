/**
 * Toast hook adapter for component usage.
 *
 * Provides a simplified interface for showing toast notifications.
 * Wraps the uiStore toast functionality with a consistent API.
 */
import { useUIStore } from '@/stores/uiStore';

interface ToastOptions {
	description: string;
	title?: string;
	duration?: number;
}

interface UseToastReturn {
	toast: (options: ToastOptions) => void;
}

export function useToast(): UseToastReturn {
	const toastApi = useUIStore((state) => state.toast);

	return {
		toast: (options: ToastOptions) => {
			toastApi.info(options.description, {
				title: options.title,
				duration: options.duration,
			});
		},
	};
}
