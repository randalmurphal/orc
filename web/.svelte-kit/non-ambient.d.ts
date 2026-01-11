
// this file is generated — do not edit it


declare module "svelte/elements" {
	export interface HTMLAttributes<T> {
		'data-sveltekit-keepfocus'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-noscroll'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-preload-code'?:
			| true
			| ''
			| 'eager'
			| 'viewport'
			| 'hover'
			| 'tap'
			| 'off'
			| undefined
			| null;
		'data-sveltekit-preload-data'?: true | '' | 'hover' | 'tap' | 'off' | undefined | null;
		'data-sveltekit-reload'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-replacestate'?: true | '' | 'off' | undefined | null;
	}
}

export {};


declare module "$app/types" {
	export interface AppTypes {
		RouteId(): "/" | "/agents" | "/board" | "/claudemd" | "/config" | "/dashboard" | "/hooks" | "/mcp" | "/prompts" | "/scripts" | "/settings" | "/skills" | "/tasks" | "/tasks/[id]" | "/tools";
		RouteParams(): {
			"/tasks/[id]": { id: string }
		};
		LayoutParams(): {
			"/": { id?: string };
			"/agents": Record<string, never>;
			"/board": Record<string, never>;
			"/claudemd": Record<string, never>;
			"/config": Record<string, never>;
			"/dashboard": Record<string, never>;
			"/hooks": Record<string, never>;
			"/mcp": Record<string, never>;
			"/prompts": Record<string, never>;
			"/scripts": Record<string, never>;
			"/settings": Record<string, never>;
			"/skills": Record<string, never>;
			"/tasks": { id?: string };
			"/tasks/[id]": { id: string };
			"/tools": Record<string, never>
		};
		Pathname(): "/" | "/agents" | "/agents/" | "/board" | "/board/" | "/claudemd" | "/claudemd/" | "/config" | "/config/" | "/dashboard" | "/dashboard/" | "/hooks" | "/hooks/" | "/mcp" | "/mcp/" | "/prompts" | "/prompts/" | "/scripts" | "/scripts/" | "/settings" | "/settings/" | "/skills" | "/skills/" | "/tasks" | "/tasks/" | `/tasks/${string}` & {} | `/tasks/${string}/` & {} | "/tools" | "/tools/";
		ResolvedPathname(): `${"" | `/${string}`}${ReturnType<AppTypes['Pathname']>}`;
		Asset(): "/fonts/inter-latin-400.woff2" | "/fonts/inter-latin-500.woff2" | "/fonts/inter-latin-600.woff2" | "/fonts/inter-latin-700.woff2" | "/fonts/jetbrains-mono-latin-400.woff2" | "/fonts/jetbrains-mono-latin-500.woff2" | "/fonts/jetbrains-mono-latin-600.woff2" | string & {};
	}
}