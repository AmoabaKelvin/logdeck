import type React from "react";
import { createContext, useContext, useEffect, useState } from "react";

import { getAuthToken, removeAuthToken, setAuthToken } from "@/lib/api-client";
import { isAuthEnabled as fetchIsAuthEnabled } from "@/lib/auth-config";
import { API_BASE_URL } from "@/types/api";

interface User {
	username: string;
	role: string;
}

interface AuthContextType {
	user: User | null;
	token: string | null;
	isAuthenticated: boolean;
	isLoading: boolean;
	isAuthEnabled: boolean;
	login: (username: string, password: string) => Promise<void>;
	logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
	const [user, setUser] = useState<User | null>(null);
	const [token, setToken] = useState<string | null>(getAuthToken);
	const [isLoading, setIsLoading] = useState(true);
	const [isAuthEnabled, setIsAuthEnabled] = useState(true);

	useEffect(() => {
		async function checkIfAuthEnabled() {
			try {
				setIsAuthEnabled(await fetchIsAuthEnabled());
			} catch (error) {
				// Status unknown - keep the default (enabled) to fail closed
				console.error("Failed to check auth status:", error);
			} finally {
				setIsLoading(false);
			}
		}

		async function verifyToken(tokenToVerify: string) {
			try {
				const response = await fetch(`${API_BASE_URL}/api/v1/auth/me`, {
					headers: {
						Authorization: `Bearer ${tokenToVerify}`,
					},
				});

				if (response.ok) {
					const data = await response.json();
					setUser(data.user);
					setToken(tokenToVerify);
					setIsAuthEnabled(true);
				} else if (response.status === 404) {
					// Auth endpoint doesn't exist - auth is disabled
					setIsAuthEnabled(false);
					removeAuthToken();
					setToken(null);
					setUser(null);
				} else if (response.status === 401) {
					// Only a 401 rejects the token. A restart or a proxy error keeps
					// it for the next request to decide.
					removeAuthToken();
					setToken(null);
					setUser(null);
				}
			} catch (error) {
				console.error("Failed to verify token:", error);
			} finally {
				setIsLoading(false);
			}
		}

		const storedToken = getAuthToken();
		if (storedToken) {
			verifyToken(storedToken);
		} else {
			checkIfAuthEnabled();
		}
	}, []);

	const login = async (username: string, password: string) => {
		try {
			const response = await fetch(`${API_BASE_URL}/api/v1/auth/login`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
				},
				body: JSON.stringify({ username, password }),
			});

			if (!response.ok) {
				const errorText = await response.text();
				throw new Error(errorText || "Login failed");
			}

			const data = await response.json();

			setAuthToken(data.token);
			setToken(data.token);
			setUser(data.user);
		} catch (error) {
			console.error("Login error:", error);
			throw error;
		}
	};

	const logout = () => {
		removeAuthToken();
		setToken(null);
		setUser(null);
	};

	const value: AuthContextType = {
		user,
		token,
		isAuthenticated: !!token && !!user,
		isLoading,
		isAuthEnabled,
		login,
		logout,
	};

	return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
	const context = useContext(AuthContext);
	if (context === undefined) {
		throw new Error("useAuth must be used within an AuthProvider");
	}
	return context;
}
