/**
 * Small confirmation toasts for the triage page, with an optional action (Undo).
 */
export interface TriageToast {
    id: number;
    message: string;
    tone: 'success' | 'error' | 'info';
    icon?: string;
    actionLabel?: string;
    action?: () => void;
}

const DEFAULT_DURATION_MS = 2200;
const MAX_TOASTS = 4;

export function useTriageToasts() {
    const toasts = ref<TriageToast[]>([]);
    const timers = new Map<number, ReturnType<typeof setTimeout>>();
    let nextId = 1;

    const dismiss = (id: number) => {
        const timer = timers.get(id);
        if (timer) clearTimeout(timer);
        timers.delete(id);
        toasts.value = toasts.value.filter((t) => t.id !== id);
    };

    const show = (
        message: string,
        options: Partial<Omit<TriageToast, 'id' | 'message'>> & { duration?: number } = {},
    ): number => {
        const id = nextId++;
        const { duration, ...rest } = options;
        toasts.value = [...toasts.value, { id, message, tone: 'success', ...rest }];
        while (toasts.value.length > MAX_TOASTS) dismiss(toasts.value[0]!.id);
        timers.set(
            id,
            setTimeout(() => dismiss(id), duration ?? DEFAULT_DURATION_MS),
        );
        return id;
    };

    const error = (message: string) =>
        show(message, { tone: 'error', icon: 'heroicons:exclamation-triangle', duration: 4000 });

    const runAction = (toast: TriageToast) => {
        dismiss(toast.id);
        toast.action?.();
    };

    onBeforeUnmount(() => {
        for (const timer of timers.values()) clearTimeout(timer);
        timers.clear();
    });

    return { toasts: readonly(toasts), show, error, dismiss, runAction };
}
