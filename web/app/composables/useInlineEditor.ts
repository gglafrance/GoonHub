/**
 * Composable for inline editing with auto-save on blur.
 * Used for title, description, and release date editing.
 */
export const useInlineEditor = <T>(options: {
    getValue: () => T;
    onSave: (value: T) => Promise<void>;
}) => {
    const editing = ref(false);
    const editValue = ref<T>(options.getValue() as T) as Ref<T>;
    const saving = ref(false);
    const saved = ref(false);
    const error = ref<string | null>(null);

    let savedTimeout: ReturnType<typeof setTimeout> | null = null;

    const startEditing = () => {
        editValue.value = options.getValue();
        editing.value = true;
    };

    const save = async () => {
        editing.value = false;
        const currentValue = options.getValue();
        if (editValue.value === currentValue) return;

        saving.value = true;
        error.value = null;

        try {
            await options.onSave(editValue.value);
            saved.value = true;
            if (savedTimeout) clearTimeout(savedTimeout);
            savedTimeout = setTimeout(() => {
                saved.value = false;
            }, 2000);
        } catch (err: unknown) {
            error.value = err instanceof Error ? err.message : 'Failed to save';
        } finally {
            saving.value = false;
        }
    };

    const cancel = () => {
        editing.value = false;
        editValue.value = options.getValue();
    };

    return {
        editing,
        editValue,
        saving,
        saved,
        error,
        startEditing,
        save,
        cancel,
    };
};
