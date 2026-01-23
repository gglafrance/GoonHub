import { useAuth } from './useAuth';

export const useApi = () => {
    const { getToken, logout } = useAuth();

    const getAuthHeaders = () => {
        const token = getToken();
        const headers: Record<string, string> = {
            'Content-Type': 'application/json',
        };

        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        return headers;
    };

    const handleResponse = async (response: Response) => {
        if (response.status === 401) {
            logout();
            throw new Error('Unauthorized');
        }

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Request failed');
        }

        return response.json();
    };

    const uploadVideo = async (file: File, title?: string) => {
        const token = getToken();
        const formData = new FormData();
        formData.append('video', file);
        if (title) {
            formData.append('title', title);
        }

        const headers: Record<string, string> = {};
        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        const response = await fetch('/api/v1/videos', {
            method: 'POST',
            body: formData,
            headers,
        });

        return handleResponse(response);
    };

    const fetchVideos = async (page: number, limit: number) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });

        const response = await fetch(`/api/v1/videos?${params}`, {
            headers: getAuthHeaders(),
        });

        return handleResponse(response);
    };

    const fetchCurrentUser = async () => {
        const response = await fetch('/api/v1/auth/me', {
            headers: getAuthHeaders(),
        });

        return handleResponse(response);
    };

    const login = async (username: string, password: string) => {
        const response = await fetch('/api/v1/auth/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ username, password }),
        });

        return handleResponse(response);
    };

    const fetchVideo = async (id: number) => {
        const response = await fetch(`/api/v1/videos/${id}`, {
            headers: getAuthHeaders(),
        });

        return handleResponse(response);
    };

    return {
        uploadVideo,
        fetchVideos,
        fetchVideo,
        fetchCurrentUser,
        login,
    };
};
