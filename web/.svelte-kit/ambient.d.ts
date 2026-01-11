
// this file is generated — do not edit it


/// <reference types="@sveltejs/kit" />

/**
 * Environment variables [loaded by Vite](https://vitejs.dev/guide/env-and-mode.html#env-files) from `.env` files and `process.env`. Like [`$env/dynamic/private`](https://svelte.dev/docs/kit/$env-dynamic-private), this module cannot be imported into client-side code. This module only includes variables that _do not_ begin with [`config.kit.env.publicPrefix`](https://svelte.dev/docs/kit/configuration#env) _and do_ start with [`config.kit.env.privatePrefix`](https://svelte.dev/docs/kit/configuration#env) (if configured).
 * 
 * _Unlike_ [`$env/dynamic/private`](https://svelte.dev/docs/kit/$env-dynamic-private), the values exported from this module are statically injected into your bundle at build time, enabling optimisations like dead code elimination.
 * 
 * ```ts
 * import { API_KEY } from '$env/static/private';
 * ```
 * 
 * Note that all environment variables referenced in your code should be declared (for example in an `.env` file), even if they don't have a value until the app is deployed:
 * 
 * ```
 * MY_FEATURE_FLAG=""
 * ```
 * 
 * You can override `.env` values from the command line like so:
 * 
 * ```sh
 * MY_FEATURE_FLAG="enabled" npm run dev
 * ```
 */
declare module '$env/static/private' {
	export const SHELL: string;
	export const npm_command: string;
	export const LSCOLORS: string;
	export const COLORTERM: string;
	export const LESS: string;
	export const WSL2_GUI_APPS_ENABLED: string;
	export const TERM_PROGRAM_VERSION: string;
	export const WSL_DISTRO_NAME: string;
	export const NODE: string;
	export const MAKE_TERMOUT: string;
	export const npm_config_local_prefix: string;
	export const GOBIN: string;
	export const NAME: string;
	export const PWD: string;
	export const LOGNAME: string;
	export const _: string;
	export const FZF_DEFAULT_COMMAND: string;
	export const REPOS_PATH: string;
	export const ZSH_TMUX_CONFIG: string;
	export const HOME: string;
	export const LANG: string;
	export const WSL_INTEROP: string;
	export const LS_COLORS: string;
	export const npm_package_version: string;
	export const _ZSH_TMUX_FIXED_CONFIG: string;
	export const VIRTUAL_ENV: string;
	export const WAYLAND_DISPLAY: string;
	export const __MISE_DIFF: string;
	export const VIRTUAL_ENV_DISABLE_PROMPT: string;
	export const GOROOT: string;
	export const MFLAGS: string;
	export const __MISE_ORIG_PATH: string;
	export const npm_lifecycle_script: string;
	export const MAKEFLAGS: string;
	export const TERM: string;
	export const npm_package_name: string;
	export const ZSH: string;
	export const __MISE_ZSH_PRECMD_RUN: string;
	export const USER: string;
	export const MAKE_TERMERR: string;
	export const RUFF_CONFIG: string;
	export const __MISE_SESSION: string;
	export const DISPLAY: string;
	export const npm_lifecycle_event: string;
	export const SHLVL: string;
	export const PAGER: string;
	export const MAKELEVEL: string;
	export const VIRTUAL_ENV_PROMPT: string;
	export const npm_config_user_agent: string;
	export const npm_execpath: string;
	export const XDG_RUNTIME_DIR: string;
	export const ZSH_TMUX_TERM: string;
	export const npm_package_json: string;
	export const WSLENV: string;
	export const BUN_INSTALL: string;
	export const MISE_SHELL: string;
	export const PATH: string;
	export const DBUS_SESSION_BUS_ADDRESS: string;
	export const HOSTTYPE: string;
	export const PULSE_SERVER: string;
	export const npm_node_execpath: string;
	export const OLDPWD: string;
	export const GOPATH: string;
	export const TERM_PROGRAM: string;
	export const NODE_ENV: string;
}

/**
 * Similar to [`$env/static/private`](https://svelte.dev/docs/kit/$env-static-private), except that it only includes environment variables that begin with [`config.kit.env.publicPrefix`](https://svelte.dev/docs/kit/configuration#env) (which defaults to `PUBLIC_`), and can therefore safely be exposed to client-side code.
 * 
 * Values are replaced statically at build time.
 * 
 * ```ts
 * import { PUBLIC_BASE_URL } from '$env/static/public';
 * ```
 */
declare module '$env/static/public' {
	
}

/**
 * This module provides access to runtime environment variables, as defined by the platform you're running on. For example if you're using [`adapter-node`](https://github.com/sveltejs/kit/tree/main/packages/adapter-node) (or running [`vite preview`](https://svelte.dev/docs/kit/cli)), this is equivalent to `process.env`. This module only includes variables that _do not_ begin with [`config.kit.env.publicPrefix`](https://svelte.dev/docs/kit/configuration#env) _and do_ start with [`config.kit.env.privatePrefix`](https://svelte.dev/docs/kit/configuration#env) (if configured).
 * 
 * This module cannot be imported into client-side code.
 * 
 * ```ts
 * import { env } from '$env/dynamic/private';
 * console.log(env.DEPLOYMENT_SPECIFIC_VARIABLE);
 * ```
 * 
 * > [!NOTE] In `dev`, `$env/dynamic` always includes environment variables from `.env`. In `prod`, this behavior will depend on your adapter.
 */
declare module '$env/dynamic/private' {
	export const env: {
		SHELL: string;
		npm_command: string;
		LSCOLORS: string;
		COLORTERM: string;
		LESS: string;
		WSL2_GUI_APPS_ENABLED: string;
		TERM_PROGRAM_VERSION: string;
		WSL_DISTRO_NAME: string;
		NODE: string;
		MAKE_TERMOUT: string;
		npm_config_local_prefix: string;
		GOBIN: string;
		NAME: string;
		PWD: string;
		LOGNAME: string;
		_: string;
		FZF_DEFAULT_COMMAND: string;
		REPOS_PATH: string;
		ZSH_TMUX_CONFIG: string;
		HOME: string;
		LANG: string;
		WSL_INTEROP: string;
		LS_COLORS: string;
		npm_package_version: string;
		_ZSH_TMUX_FIXED_CONFIG: string;
		VIRTUAL_ENV: string;
		WAYLAND_DISPLAY: string;
		__MISE_DIFF: string;
		VIRTUAL_ENV_DISABLE_PROMPT: string;
		GOROOT: string;
		MFLAGS: string;
		__MISE_ORIG_PATH: string;
		npm_lifecycle_script: string;
		MAKEFLAGS: string;
		TERM: string;
		npm_package_name: string;
		ZSH: string;
		__MISE_ZSH_PRECMD_RUN: string;
		USER: string;
		MAKE_TERMERR: string;
		RUFF_CONFIG: string;
		__MISE_SESSION: string;
		DISPLAY: string;
		npm_lifecycle_event: string;
		SHLVL: string;
		PAGER: string;
		MAKELEVEL: string;
		VIRTUAL_ENV_PROMPT: string;
		npm_config_user_agent: string;
		npm_execpath: string;
		XDG_RUNTIME_DIR: string;
		ZSH_TMUX_TERM: string;
		npm_package_json: string;
		WSLENV: string;
		BUN_INSTALL: string;
		MISE_SHELL: string;
		PATH: string;
		DBUS_SESSION_BUS_ADDRESS: string;
		HOSTTYPE: string;
		PULSE_SERVER: string;
		npm_node_execpath: string;
		OLDPWD: string;
		GOPATH: string;
		TERM_PROGRAM: string;
		NODE_ENV: string;
		[key: `PUBLIC_${string}`]: undefined;
		[key: `${string}`]: string | undefined;
	}
}

/**
 * Similar to [`$env/dynamic/private`](https://svelte.dev/docs/kit/$env-dynamic-private), but only includes variables that begin with [`config.kit.env.publicPrefix`](https://svelte.dev/docs/kit/configuration#env) (which defaults to `PUBLIC_`), and can therefore safely be exposed to client-side code.
 * 
 * Note that public dynamic environment variables must all be sent from the server to the client, causing larger network requests — when possible, use `$env/static/public` instead.
 * 
 * ```ts
 * import { env } from '$env/dynamic/public';
 * console.log(env.PUBLIC_DEPLOYMENT_SPECIFIC_VARIABLE);
 * ```
 */
declare module '$env/dynamic/public' {
	export const env: {
		[key: `PUBLIC_${string}`]: string | undefined;
	}
}
