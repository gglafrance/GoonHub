import type { HomepageConfig } from '~/types/homepage';
import type { ParsingRulesSettings } from '~/types/parsing-rules';

/**
 * User settings API operations: player, app, tags, account, homepage, parsing rules.
 */
export const useApiSettings = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const fetchSettings = async () => {
        const response = await fetch('/api/v1/settings', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updatePlayerSettings = async (settings: {
        autoplay: boolean;
        default_volume: number;
        loop: boolean;
    }) => {
        const response = await fetch('/api/v1/settings/player', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(settings),
        });
        return handleResponse(response);
    };

    const updateAppSettings = async (settings: {
        videos_per_page: number;
        default_sort_order: string;
        marker_thumbnail_cycling: boolean;
    }) => {
        const response = await fetch('/api/v1/settings/app', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(settings),
        });
        return handleResponse(response);
    };

    const updateTagSettings = async (settings: { default_tag_sort: string }) => {
        const response = await fetch('/api/v1/settings/tags', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(settings),
        });
        return handleResponse(response);
    };

    const changePassword = async (currentPassword: string, newPassword: string) => {
        const response = await fetch('/api/v1/settings/password', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
        });
        return handleResponse(response);
    };

    const changeUsername = async (username: string) => {
        const response = await fetch('/api/v1/settings/username', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ username }),
        });
        return handleResponse(response);
    };

    const getHomepageConfig = async (): Promise<HomepageConfig> => {
        const response = await fetch('/api/v1/settings/homepage', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateHomepageConfig = async (config: HomepageConfig): Promise<HomepageConfig> => {
        const response = await fetch('/api/v1/settings/homepage', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    const getParsingRules = async (): Promise<ParsingRulesSettings> => {
        const response = await fetch('/api/v1/settings/parsing-rules', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateParsingRules = async (
        settings: ParsingRulesSettings,
    ): Promise<ParsingRulesSettings> => {
        const response = await fetch('/api/v1/settings/parsing-rules', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(settings),
        });
        return handleResponse(response);
    };

    return {
        fetchSettings,
        updatePlayerSettings,
        updateAppSettings,
        updateTagSettings,
        changePassword,
        changeUsername,
        getHomepageConfig,
        updateHomepageConfig,
        getParsingRules,
        updateParsingRules,
    };
};
