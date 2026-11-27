import type { NavLayout, NavLayoutView } from '~/types/navLayout';
import type { ParsingRulesSettings } from '~/types/parsing-rules';
import type { UserSettings } from '~/types/settings';
import type { SettingsPatch } from '~/utils/settingsPatch';

/**
 * User settings API operations: unified settings (full PUT or partial PATCH), account,
 * parsing rules, header layout (docs/NAVIGATION.md).
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

    const updateAllSettings = async (settings: UserSettings): Promise<UserSettings> => {
        const response = await fetch('/api/v1/settings', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(settings),
        });
        return handleResponse(response);
    };

    // Partial update: only the given settings change; returns all the settings
    const patchSettings = async (patch: SettingsPatch): Promise<UserSettings> => {
        const response = await fetch('/api/v1/settings', {
            method: 'PATCH',
            headers: getAuthHeaders(),
            body: JSON.stringify(patch),
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

    // Header layout: GET answers the stored or default layout without the forbidden items, PUT
    // stores a layout (forbidden items left out), DELETE goes back to the default layout
    const getNavLayout = async (): Promise<NavLayoutView> => {
        const response = await fetch('/api/v1/settings/navigation', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const saveNavLayout = async (layout: NavLayout): Promise<NavLayoutView> => {
        const response = await fetch('/api/v1/settings/navigation', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ layout }),
        });
        return handleResponse(response);
    };

    const resetNavLayout = async (): Promise<NavLayoutView> => {
        const response = await fetch('/api/v1/settings/navigation', {
            method: 'DELETE',
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    return {
        fetchSettings,
        getNavLayout,
        saveNavLayout,
        resetNavLayout,
        updateAllSettings,
        patchSettings,
        changePassword,
        changeUsername,
        getParsingRules,
        updateParsingRules,
    };
};
