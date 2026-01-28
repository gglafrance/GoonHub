/**
 * Core API utilities shared across all domain-specific API composables.
 * Provides fetch wrapper with authentication and error handling.
 */
export const useApiCore = () => {
    const authStore = useAuthStore();

    // Common fetch options that include credentials for cookie-based auth
    const fetchOptions = (): RequestInit => ({
        credentials: 'include', // Send HTTP-only cookies
    });

    const getAuthHeaders = () => {
        const headers: Record<string, string> = {
            'Content-Type': 'application/json',
        };
        // No longer need to manually add Authorization header
        // as authentication is handled via HTTP-only cookies
        return headers;
    };

    const handleResponse = async (response: Response) => {
        if (response.status === 401) {
            authStore.logout();
            throw new Error('Unauthorized');
        }

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Request failed');
        }

        return response.json();
    };

    // Handle responses that may return 204 No Content
    const handleResponseWithNoContent = async (response: Response) => {
        if (response.status === 401) {
            authStore.logout();
            throw new Error('Unauthorized');
        }
        if (!response.ok && response.status !== 204) {
            const error = await response.json();
            throw new Error(error.error || 'Request failed');
        }
    };

    return {
        fetchOptions,
        getAuthHeaders,
        handleResponse,
        handleResponseWithNoContent,
    };
};
