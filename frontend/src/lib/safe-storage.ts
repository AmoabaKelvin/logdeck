// localStorage can be blocked or full. A preference that fails to persist
// should not take the page down with it.
export function readStorage(key: string) {
	try {
		return localStorage.getItem(key);
	} catch {
		return null;
	}
}

export function writeStorage(key: string, value: string) {
	try {
		localStorage.setItem(key, value);
	} catch {
		// The in-memory state still holds for this session.
	}
}
