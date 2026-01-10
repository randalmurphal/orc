
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
	export const COREPACK_ENABLE_AUTO_PIN: string;
	export const SESSION_MANAGER: string;
	export const npm_config_userconfig: string;
	export const COLORTERM: string;
	export const XDG_CONFIG_DIRS: string;
	export const npm_config_cache: string;
	export const LESS: string;
	export const XDG_SESSION_PATH: string;
	export const XDG_MENU_PREFIX: string;
	export const TERM_PROGRAM_VERSION: string;
	export const GTK_IM_MODULE: string;
	export const MACHTYPE: string;
	export const G_BROKEN_FILENAMES: string;
	export const HISTSIZE: string;
	export const HOSTNAME: string;
	export const ICEAUTHORITY: string;
	export const FROM_HEADER: string;
	export const MINICOM: string;
	export const NODE: string;
	export const WEZTERM_CONFIG_DIR: string;
	export const AUDIODRIVER: string;
	export const JRE_HOME: string;
	export const SSH_AUTH_SOCK: string;
	export const XDG_DATA_HOME: string;
	export const CPU: string;
	export const XDG_CONFIG_HOME: string;
	export const COLOR: string;
	export const LOCALE_ARCHIVE_2_27: string;
	export const npm_config_local_prefix: string;
	export const WEZTERM_EXECUTABLE: string;
	export const XMODIFIERS: string;
	export const DESKTOP_SESSION: string;
	export const SSH_AGENT_PID: string;
	export const __ETC_PROFILE_NIX_SOURCED: string;
	export const GTK_RC_FILES: string;
	export const npm_config_globalconfig: string;
	export const GPG_TTY: string;
	export const EDITOR: string;
	export const GTK_MODULES: string;
	export const XDG_SEAT: string;
	export const PWD: string;
	export const NIX_PROFILES: string;
	export const QEMU_AUDIO_DRV: string;
	export const LOGNAME: string;
	export const XDG_SESSION_DESKTOP: string;
	export const XDG_SESSION_TYPE: string;
	export const MANPATH: string;
	export const NIX_PATH: string;
	export const npm_config_init_module: string;
	export const SYSTEMD_EXEC_PID: string;
	export const VIPSHOME: string;
	export const _: string;
	export const XAUTHORITY: string;
	export const DESKTOP_STARTUP_ID: string;
	export const NoDefaultCurrentDirectoryInExePath: string;
	export const LS_OPTIONS: string;
	export const FZF_DEFAULT_COMMAND: string;
	export const REPOS_PATH: string;
	export const CLAUDECODE: string;
	export const ZSH_TMUX_CONFIG: string;
	export const XKEYSYMDB: string;
	export const GTK2_RC_FILES: string;
	export const XNLSPATH: string;
	export const HOME: string;
	export const SSH_ASKPASS: string;
	export const LANG: string;
	export const WEZTERM_UNIX_SOCKET: string;
	export const LS_COLORS: string;
	export const XDG_CURRENT_DESKTOP: string;
	export const npm_package_version: string;
	export const _ZSH_TMUX_FIXED_CONFIG: string;
	export const VIRTUAL_ENV: string;
	export const PYTHONSTARTUP: string;
	export const SSL_CERT_DIR: string;
	export const NIX_SSL_CERT_FILE: string;
	export const VIRTUAL_ENV_DISABLE_PROMPT: string;
	export const OSTYPE: string;
	export const XDG_SEAT_PATH: string;
	export const QT_IM_SWITCHER: string;
	export const LESS_ADVANCED_PREPROCESSOR: string;
	export const INVOCATION_ID: string;
	export const MANAGERPID: string;
	export const INIT_CWD: string;
	export const XSESSION_IS_UP: string;
	export const KDE_SESSION_UID: string;
	export const XDG_CACHE_HOME: string;
	export const npm_lifecycle_script: string;
	export const MOZ_GMP_PATH: string;
	export const npm_config_npm_version: string;
	export const LESSCLOSE: string;
	export const XDG_SESSION_CLASS: string;
	export const TERM: string;
	export const npm_package_name: string;
	export const ZSH: string;
	export const G_FILENAME_ENCODING: string;
	export const HOST: string;
	export const npm_config_prefix: string;
	export const XAUTHLOCALHOSTNAME: string;
	export const LESSOPEN: string;
	export const USER: string;
	export const RUFF_CONFIG: string;
	export const KDE_SESSION_VERSION: string;
	export const MORE: string;
	export const CSHEDIT: string;
	export const DISPLAY: string;
	export const npm_lifecycle_event: string;
	export const SHLVL: string;
	export const WINDOWMANAGER: string;
	export const GIT_EDITOR: string;
	export const PAGER: string;
	export const QT_IM_MODULE: string;
	export const XDG_VTNR: string;
	export const XDG_SESSION_ID: string;
	export const VIRTUAL_ENV_PROMPT: string;
	export const npm_config_user_agent: string;
	export const TERMINFO_DIRS: string;
	export const OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE: string;
	export const XDG_STATE_HOME: string;
	export const npm_execpath: string;
	export const WEZTERM_CONFIG_FILE: string;
	export const XDG_RUNTIME_DIR: string;
	export const SSL_CERT_FILE: string;
	export const ZSH_TMUX_TERM: string;
	export const CLAUDE_CODE_ENTRYPOINT: string;
	export const NIX_XDG_DESKTOP_PORTAL_DIR: string;
	export const DEBUGINFOD_URLS: string;
	export const npm_package_json: string;
	export const BUN_INSTALL: string;
	export const JOURNAL_STREAM: string;
	export const MANPATHISSET: string;
	export const XDG_DATA_DIRS: string;
	export const KDE_FULL_SESSION: string;
	export const npm_config_noproxy: string;
	export const CONFIG_SITE: string;
	export const VENDOR: string;
	export const PATH: string;
	export const npm_config_node_gyp: string;
	export const DBUS_SESSION_BUS_ADDRESS: string;
	export const PROFILEREAD: string;
	export const npm_config_global_prefix: string;
	export const KDE_APPLICATIONS_AS_SCOPE: string;
	export const MAIL: string;
	export const HOSTTYPE: string;
	export const NODE_VERSION: string;
	export const npm_node_execpath: string;
	export const LESSKEY: string;
	export const WEZTERM_PANE: string;
	export const OLDPWD: string;
	export const TERM_PROGRAM: string;
	export const WEZTERM_EXECUTABLE_DIR: string;
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
		COREPACK_ENABLE_AUTO_PIN: string;
		SESSION_MANAGER: string;
		npm_config_userconfig: string;
		COLORTERM: string;
		XDG_CONFIG_DIRS: string;
		npm_config_cache: string;
		LESS: string;
		XDG_SESSION_PATH: string;
		XDG_MENU_PREFIX: string;
		TERM_PROGRAM_VERSION: string;
		GTK_IM_MODULE: string;
		MACHTYPE: string;
		G_BROKEN_FILENAMES: string;
		HISTSIZE: string;
		HOSTNAME: string;
		ICEAUTHORITY: string;
		FROM_HEADER: string;
		MINICOM: string;
		NODE: string;
		WEZTERM_CONFIG_DIR: string;
		AUDIODRIVER: string;
		JRE_HOME: string;
		SSH_AUTH_SOCK: string;
		XDG_DATA_HOME: string;
		CPU: string;
		XDG_CONFIG_HOME: string;
		COLOR: string;
		LOCALE_ARCHIVE_2_27: string;
		npm_config_local_prefix: string;
		WEZTERM_EXECUTABLE: string;
		XMODIFIERS: string;
		DESKTOP_SESSION: string;
		SSH_AGENT_PID: string;
		__ETC_PROFILE_NIX_SOURCED: string;
		GTK_RC_FILES: string;
		npm_config_globalconfig: string;
		GPG_TTY: string;
		EDITOR: string;
		GTK_MODULES: string;
		XDG_SEAT: string;
		PWD: string;
		NIX_PROFILES: string;
		QEMU_AUDIO_DRV: string;
		LOGNAME: string;
		XDG_SESSION_DESKTOP: string;
		XDG_SESSION_TYPE: string;
		MANPATH: string;
		NIX_PATH: string;
		npm_config_init_module: string;
		SYSTEMD_EXEC_PID: string;
		VIPSHOME: string;
		_: string;
		XAUTHORITY: string;
		DESKTOP_STARTUP_ID: string;
		NoDefaultCurrentDirectoryInExePath: string;
		LS_OPTIONS: string;
		FZF_DEFAULT_COMMAND: string;
		REPOS_PATH: string;
		CLAUDECODE: string;
		ZSH_TMUX_CONFIG: string;
		XKEYSYMDB: string;
		GTK2_RC_FILES: string;
		XNLSPATH: string;
		HOME: string;
		SSH_ASKPASS: string;
		LANG: string;
		WEZTERM_UNIX_SOCKET: string;
		LS_COLORS: string;
		XDG_CURRENT_DESKTOP: string;
		npm_package_version: string;
		_ZSH_TMUX_FIXED_CONFIG: string;
		VIRTUAL_ENV: string;
		PYTHONSTARTUP: string;
		SSL_CERT_DIR: string;
		NIX_SSL_CERT_FILE: string;
		VIRTUAL_ENV_DISABLE_PROMPT: string;
		OSTYPE: string;
		XDG_SEAT_PATH: string;
		QT_IM_SWITCHER: string;
		LESS_ADVANCED_PREPROCESSOR: string;
		INVOCATION_ID: string;
		MANAGERPID: string;
		INIT_CWD: string;
		XSESSION_IS_UP: string;
		KDE_SESSION_UID: string;
		XDG_CACHE_HOME: string;
		npm_lifecycle_script: string;
		MOZ_GMP_PATH: string;
		npm_config_npm_version: string;
		LESSCLOSE: string;
		XDG_SESSION_CLASS: string;
		TERM: string;
		npm_package_name: string;
		ZSH: string;
		G_FILENAME_ENCODING: string;
		HOST: string;
		npm_config_prefix: string;
		XAUTHLOCALHOSTNAME: string;
		LESSOPEN: string;
		USER: string;
		RUFF_CONFIG: string;
		KDE_SESSION_VERSION: string;
		MORE: string;
		CSHEDIT: string;
		DISPLAY: string;
		npm_lifecycle_event: string;
		SHLVL: string;
		WINDOWMANAGER: string;
		GIT_EDITOR: string;
		PAGER: string;
		QT_IM_MODULE: string;
		XDG_VTNR: string;
		XDG_SESSION_ID: string;
		VIRTUAL_ENV_PROMPT: string;
		npm_config_user_agent: string;
		TERMINFO_DIRS: string;
		OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE: string;
		XDG_STATE_HOME: string;
		npm_execpath: string;
		WEZTERM_CONFIG_FILE: string;
		XDG_RUNTIME_DIR: string;
		SSL_CERT_FILE: string;
		ZSH_TMUX_TERM: string;
		CLAUDE_CODE_ENTRYPOINT: string;
		NIX_XDG_DESKTOP_PORTAL_DIR: string;
		DEBUGINFOD_URLS: string;
		npm_package_json: string;
		BUN_INSTALL: string;
		JOURNAL_STREAM: string;
		MANPATHISSET: string;
		XDG_DATA_DIRS: string;
		KDE_FULL_SESSION: string;
		npm_config_noproxy: string;
		CONFIG_SITE: string;
		VENDOR: string;
		PATH: string;
		npm_config_node_gyp: string;
		DBUS_SESSION_BUS_ADDRESS: string;
		PROFILEREAD: string;
		npm_config_global_prefix: string;
		KDE_APPLICATIONS_AS_SCOPE: string;
		MAIL: string;
		HOSTTYPE: string;
		NODE_VERSION: string;
		npm_node_execpath: string;
		LESSKEY: string;
		WEZTERM_PANE: string;
		OLDPWD: string;
		TERM_PROGRAM: string;
		WEZTERM_EXECUTABLE_DIR: string;
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
