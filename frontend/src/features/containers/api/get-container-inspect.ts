import { authenticatedFetch } from "@/lib/api-client";
import { API_BASE_URL } from "@/types/api";

/**
 * The slice of Docker's inspect payload the UI reads. The endpoint returns the
 * whole thing; typing only what we use keeps the surface honest and avoids
 * mirroring Docker's API shape into the app.
 */
export interface HealthProbe {
	Start: string;
	End: string;
	ExitCode: number;
	Output: string;
}

export interface ContainerInspect {
	Id: string;
	Created: string;
	RestartCount: number;
	Image: string;
	Platform?: string;
	State: {
		Status: string;
		Running: boolean;
		Paused: boolean;
		Restarting: boolean;
		OOMKilled: boolean;
		Dead: boolean;
		Pid: number;
		ExitCode: number;
		Error: string;
		StartedAt: string;
		FinishedAt: string;
		Health?: {
			Status: string;
			FailingStreak: number;
			Log: HealthProbe[] | null;
		};
	};
	Mounts: {
		Type: string;
		Name?: string;
		Source: string;
		Destination: string;
		Driver?: string;
		Mode: string;
		RW: boolean;
	}[];
	Config: {
		Hostname?: string;
		User?: string;
		WorkingDir?: string;
		Entrypoint?: string[] | null;
		Cmd?: string[] | null;
		ExposedPorts?: Record<string, unknown>;
		Healthcheck?: {
			Test?: string[];
			// Docker reports these in nanoseconds.
			Interval?: number;
			Timeout?: number;
			StartPeriod?: number;
			Retries?: number;
		} | null;
	};
	HostConfig: {
		Memory?: number;
		NanoCpus?: number;
		Privileged?: boolean;
		RestartPolicy?: { Name: string; MaximumRetryCount: number };
	};
	NetworkSettings: {
		Ports?: Record<string, { HostIp: string; HostPort: string }[] | null>;
		Networks?: Record<
			string,
			{
				IPAddress?: string;
				Gateway?: string;
				MacAddress?: string;
				Aliases?: string[] | null;
				NetworkID?: string;
			}
		>;
	};
}

export async function getContainerInspect(
	id: string,
	host: string,
): Promise<ContainerInspect> {
	const response = await authenticatedFetch(
		`${API_BASE_URL}/api/v1/containers/${encodeURIComponent(id)}/?host=${encodeURIComponent(host)}`,
	);

	if (!response.ok) {
		throw new Error("Failed to inspect container");
	}

	const body = (await response.json()) as { container: ContainerInspect };
	return body.container;
}
