import type { ParsingRulesSettings } from '~/types/parsing-rules';
import type { UserSettings } from '~/types/settings';
import type { SettingsPatch } from '~/utils/settingsPatch';

/**
 * User settings API operations: unified settings (full PUT or partial PATCH), account,
 * parsing rules.
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

    return {
        fetchSettings,
        updateAllSettings,
        patchSettings,
        changePassword,
        changeUsername,
        getParsingRules,
        updateParsingRules,
    };
};
