export const useSettingsMessage = () => {
    const message = ref('');
    const error = ref('');

    const clearMessages = () => {
        message.value = '';
        error.value = '';
    };

    const setMessage = (msg: string) => {
        clearMessages();
        message.value = msg;
    };

    const setError = (e: unknown, fallback?: string) => {
        clearMessages();
        if (typeof e === 'string') {
            error.value = e;
        } else {
            error.value = e instanceof Error ? e.message : (fallback ?? 'Unknown error');
        }
    };

    return { message, error, clearMessages, setMessage, setError };
};
