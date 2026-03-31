import { useEffect, useState } from 'react';

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
