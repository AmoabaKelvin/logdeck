import { toast } from "sonner";

/** Shared mutate callbacks: toast the server message on success, the error on failure. */
export const showResultToast = {
	onSuccess: (message: string) => toast.success(message),
	onError: (error: Error) => toast.error(error.message),
};
