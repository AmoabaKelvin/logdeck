// Use empty string for same-origin requests when frontend is served by backend
// This allows the browser to use relative paths (e.g., /api/v1/containers)
// Falls back to localhost:8080 for local development with separate servers
export const API_BASE_URL = import.meta.env.VITE_API_URL || "";
