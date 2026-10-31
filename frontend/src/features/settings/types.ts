export type ConfigSource = "file" | "env" | "default" | "mixed";

export interface DockerHost {
	name: string;
	host: string;
	source: ConfigSource;
}

export interface PlatformHost {
	hostName: string;
	apiURL: string;
	apiToken: string;
	source: ConfigSource;
}

export type CoolifyHost = PlatformHost;

export interface DokployHost extends PlatformHost {
	serverId: string;
}
export interface DokployHostsConfig {
	source: ConfigSource;
	hosts: DokployHost[];
	revision?: string;
}

export interface DockerHostsConfig {
	source: ConfigSource;
	hosts: DockerHost[];
	/** Echoed back on save so a concurrent edit is rejected instead of overwritten. */
	revision?: string;
}

export interface CoolifyHostsConfig {
	source: ConfigSource;
	hosts: CoolifyHost[];
	revision?: string;
}

export interface ReadOnlyConfig {
	source: ConfigSource;
	value: boolean;
}

export interface AuthConfig {
	source: ConfigSource;
	enabled: boolean;
	/** Absent on servers older than OIDC sign-in. */
	oidc?: OIDCConfig;
}

export interface OIDCConfig {
	source: ConfigSource;
	enabled: boolean;
	issuer?: string;
	clientId?: string;
	/** Masked when one is stored; sending the mask back keeps it. */
	clientSecret?: string;
	redirectUrl?: string;
	groupsClaim?: string;
	adminGroups?: string[];
	readGroups?: string[];
	/** Given to users in none of the groups; empty refuses them. */
	defaultRole?: "" | UserRole;
}

/** Roles mean the same as API token scopes. */
export type UserRole = "admin" | "read";

export interface ManagedUser {
	username: string;
	role: UserRole;
	/** "env" for the admin set by ADMIN_USERNAME, which can't be changed here. */
	source: ConfigSource;
	createdAt?: string;
}

export interface UsersResponse {
	users: ManagedUser[];
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
	// Days a removed container's logs outlive it; 0 keeps them until a cap
	// evicts them.
	removedDays: number;
	removedDaysSource: ConfigSource;
}

export interface SettingsResponse {
	dockerHosts: DockerHostsConfig;
	coolifyHosts: CoolifyHostsConfig;
	dokployHosts?: DokployHostsConfig;
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

/** Counts of what a backup, or this instance, holds. */
export interface BackupContents {
	dockerHosts: number;
	coolifyHosts: number;
	dokployHosts: number;
	alertRules: number;
	alertChannels: number;
	alertHistory: number;
	auditEntries: number;
	apiTokens: number;
	logsBytes: number;
}

export interface BackupManifest {
	format: number;
	logdeckVersion: string;
	createdAt: string;
	includesLogs: boolean;
	includesSecrets: boolean;
	contents: BackupContents;
}

export interface BackupEntry {
	name: string;
	sizeBytes: number;
	createdAt: string;
	scheduled: boolean;
	encrypted: boolean;
	/** Only unencrypted backups reveal their manifest without the passphrase. */
	manifest?: BackupManifest;
}

export interface BackupSchedule {
	/** 0 turns scheduled backups off. */
	intervalHours: number;
	keep: number;
	includeLogs: boolean;
	includeSecrets: boolean;
}

export interface BackupResult {
	name?: string;
	at: string;
	sizeBytes?: number;
	scheduled: boolean;
	error?: string;
}

/** What restoring a checked backup would replace. */
export interface RestorePreview {
	manifest: BackupManifest;
	encrypted: boolean;
	current: BackupContents;
	missingCredentials: number;
	warnings: string[];
}

export interface BackupStatus {
	dir: string;
	running: boolean;
	last?: BackupResult;
	schedule: BackupSchedule;
	nextRunAt?: string;
	/** Scheduled backups are encrypted when BACKUP_PASSPHRASE is set. */
	scheduleEncrypted: boolean;
	logsAvailable: boolean;
	backups: BackupEntry[];
	listError?: string;
	/** A confirmed restore waiting for the next start. */
	pendingRestore?: RestorePreview;
}
