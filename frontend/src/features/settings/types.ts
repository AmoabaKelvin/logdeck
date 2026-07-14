export type ConfigSource = "file" | "env" | "default" | "mixed";

export interface DockerHost {
	name: string;
	host: string;
	source: ConfigSource;
}

export interface CoolifyHost {
	hostName: string;
	apiURL: string;
	apiToken: string;
	source: ConfigSource;
}

export interface DockerHostsConfig {
	source: ConfigSource;
	hosts: DockerHost[];
}

export interface CoolifyHostsConfig {
	source: ConfigSource;
	hosts: CoolifyHost[];
}

export interface ReadOnlyConfig {
	source: ConfigSource;
	value: boolean;
}

export interface AuthConfig {
	source: ConfigSource;
	enabled: boolean;
	adminUsername?: string;
}

// Unlike the other categories, each log store field is overridden
// independently, so it carries its own source rather than one for the section.
export interface LogStoreConfig {
	enabled: boolean;
	enabledSource: ConfigSource;
	perContainerMB: number;
	perContainerMBSource: ConfigSource;
	totalMB: number;
	totalMBSource: ConfigSource;
}

export interface SettingsResponse {
	dockerHosts: DockerHostsConfig;
	coolifyHosts: CoolifyHostsConfig;
	readOnly: ReadOnlyConfig;
	auth: AuthConfig;
	/** Absent on servers older than the editable retention caps. */
	logStore?: LogStoreConfig;
}

export type APITokenScope = "admin" | "read";

export interface APIToken {
	name: string;
	prefix: string;
	createdAt: string;
	scope: APITokenScope;
}

export interface APITokensResponse {
	tokens: APIToken[];
}

export interface CreatedAPIToken extends APIToken {
	/** The full token, returned exactly once at creation time. */
	token: string;
}

export interface TestConnectionResult {
	success: boolean;
	message: string;
	dockerVersion?: string;
	/** "Docker" or "Podman", when the server could identify the engine. */
	engine?: string;
	engineVersion?: string;
}
