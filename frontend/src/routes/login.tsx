import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { z } from "zod";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAuth } from "@/contexts/auth-context";
import { safeRedirect } from "@/lib/api-client";
import { getAuthConfig } from "@/lib/auth-config";
import { API_BASE_URL } from "@/types/api";

// Where to go after signing in; set when a page sent the user here. The OIDC
// callback adds oidc_error when a sign-in through the provider fails.
const loginSearchSchema = z
	.object({
		redirect: z.string().optional(),
		oidc_error: z.string().optional(),
	})
	.catch({});

// The server sends a code, not text, so a crafted link can't put words here.
function oidcErrorMessage(code: string): string {
	switch (code) {
		case "denied":
			return "Your account is not allowed to sign in to LogDeck. Ask an admin to map one of your groups to a role.";
		case "expired":
			return "The sign-in took too long or was started in another browser. Please try again.";
		case "cancelled":
			return "Sign-in was cancelled at the identity provider.";
		default:
			return "Signing in with SSO failed. The server log has the details.";
	}
}

// The OIDC callback lands on this page with the session token in the fragment.
function fragmentToken(): string | null {
	return new URLSearchParams(window.location.hash.slice(1)).get("oidc_token");
}

// A random value kept in this tab while it signs in through the provider. The
// callback hands it back with the token, so a token in a link someone else
// sent, which comes without it, is refused instead of signing in as them.
const OIDC_CHECK_KEY = "logdeck_oidc_check";

export const Route = createFileRoute("/login")({
	validateSearch: loginSearchSchema.parse,
	component: LoginPage,
});

function LoginPage() {
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [error, setError] = useState("");
	const [isLoading, setIsLoading] = useState(() => fragmentToken() !== null);

	const [oidcEnabled, setOidcEnabled] = useState(false);

	const { login, loginWithToken } = useAuth();
	const { redirect, oidc_error: oidcError } = Route.useSearch();
	const navigate = useNavigate();

	useEffect(() => {
		getAuthConfig()
			.then((config) => setOidcEnabled(config.oidcEnabled))
			.catch(() => setOidcEnabled(false));
	}, []);

	// Take the token out of the address bar before anything else can read it.
	// Clearing it also makes this run once, however often the effect fires.
	useEffect(() => {
		const token = fragmentToken();
		if (!token) return;
		const check = new URLSearchParams(window.location.hash.slice(1)).get(
			"check",
		);
		const expected = sessionStorage.getItem(OIDC_CHECK_KEY);
		sessionStorage.removeItem(OIDC_CHECK_KEY);
		window.history.replaceState(
			null,
			"",
			window.location.pathname + window.location.search,
		);
		void (async () => {
			if (!expected || check !== expected) {
				setError(
					"This sign-in link was not started here. Please sign in again.",
				);
				setIsLoading(false);
				return;
			}
			try {
				await loginWithToken(token);
				navigate({ href: safeRedirect(redirect) });
			} catch (err) {
				setError(err instanceof Error ? err.message : "Sign-in failed.");
				setIsLoading(false);
			}
		})();
	}, [loginWithToken, navigate, redirect]);

	const shownError = error || (oidcError ? oidcErrorMessage(oidcError) : "");

	function startOidc() {
		// getRandomValues, unlike randomUUID, also works over plain HTTP.
		const check = Array.from(crypto.getRandomValues(new Uint8Array(16)), (b) =>
			b.toString(16).padStart(2, "0"),
		).join("");
		sessionStorage.setItem(OIDC_CHECK_KEY, check);
		const query = new URLSearchParams({
			redirect: safeRedirect(redirect),
			check,
		});
		window.location.assign(`${API_BASE_URL}/api/v1/auth/oidc/login?${query}`);
	}

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		setError("");
		setIsLoading(true);

		try {
			await login(username, password);
			navigate({ href: safeRedirect(redirect) });
		} catch (err) {
			setError(
				err instanceof Error
					? err.message
					: "Login failed. Please check your credentials.",
			);
		} finally {
			setIsLoading(false);
		}
	};

	return (
		<div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-gray-50 to-gray-100 dark:from-gray-900 dark:to-gray-800">
			<div className="absolute top-4 right-4">
				<ThemeToggle />
			</div>
			<Card className="w-full max-w-md p-8 space-y-6 shadow-xl">
				<div className="text-center space-y-2">
					<h1 className="text-4xl font-bold tracking-tight">LogDeck</h1>
					<p className="text-sm text-muted-foreground">
						Container management shouldn't be that hard
					</p>
				</div>

				<form onSubmit={handleSubmit} className="space-y-4">
					<div className="space-y-2">
						<Label htmlFor="username">Username</Label>
						<Input
							id="username"
							type="text"
							value={username}
							onChange={(e) => setUsername(e.target.value)}
							placeholder="Enter your username"
							required
							autoComplete="username"
							disabled={isLoading}
						/>
					</div>

					<div className="space-y-2">
						<Label htmlFor="password">Password</Label>
						<Input
							id="password"
							type="password"
							value={password}
							onChange={(e) => setPassword(e.target.value)}
							placeholder="Enter your password"
							required
							autoComplete="current-password"
							disabled={isLoading}
						/>
					</div>

					{shownError && (
						<div className="p-3 text-sm text-red-600 bg-red-50 dark:bg-red-900/20 dark:text-red-400 rounded-md border border-red-200 dark:border-red-800">
							{shownError}
						</div>
					)}

					<Button type="submit" className="w-full" disabled={isLoading}>
						{isLoading ? "Signing in..." : "Sign in"}
					</Button>
				</form>

				{oidcEnabled && (
					<Button
						type="button"
						variant="outline"
						className="w-full"
						onClick={startOidc}
						disabled={isLoading}
					>
						Sign in with SSO
					</Button>
				)}

				<div className="text-center text-xs text-muted-foreground">
					<p>How hard can it be?</p>
				</div>
			</Card>
		</div>
	);
}
