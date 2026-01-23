export const useSettingsMessage = () => {
    const message = ref('');
    const error = ref('');

    const clearMessages = () => {
        message.value = '';
        error.value = '';
    };

    const setError = (e: unknown, fallback: string) => {
        clearMessages();
        error.value = e instanceof Error ? e.message : fallback;
    };

    return { message, error, clearMessages, setError };
};
