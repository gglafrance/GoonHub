import type { ShortsConfig } from '~/types/shorts';

/** What the Save Short buttons show: whether they work, and their tooltip. */
export interface SaveShortState {
    valid: boolean;
    reason: string;
    warning: string | null;
}

/**
 * The part of the player's A/B loop the short editor reads. The points are refs
 * at runtime; a template ref's type unwraps them, so either is accepted.
 */
interface ABLoopHandle {
    pointA: MaybeRef<number | null>;
    pointB: MaybeRef<number | null>;
    clear: () => void;
}

export const SHORT_EDITOR_KEY = 'shortEditor';

const MIN_SHORT_SECONDS = 1;

/** 01:23, or 1:02:03 from an hour on. */
export function formatShortClock(seconds: number): string {
    const total = Math.max(0, Math.floor(seconds));
    const h = Math.floor(total / 3600);
    const m = Math.floor((total % 3600) / 60);
    const s = total % 60;
    if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
    return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}

/** A length such as 0:22 or 1:34. */
export function formatShortSpan(seconds: number): string {
    const total = Math.max(0, Math.round(seconds));
    const h = Math.floor(total / 3600);
    const m = Math.floor((total % 3600) / 60);
    const s = total % 60;
    if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
    return `${m}:${String(s).padStart(2, '0')}`;
}

/**
 * State for cutting a short out of the scene on the watch page. The page
 * creates one and provides it; the player, the Shorts tab and the create modal
 * all read it. A and B come from the player's A/B loop.
 */
export const useShortEditor = (abLoop: ComputedRef<ABLoopHandle | null | undefined>) => {
    const { getShortsConfig } = useApiShorts();

    const active = ref(false);
    const modalOpen = ref(false);
    const saving = ref(false);
    const lastTaskId = ref<string | null>(null);
    const config = ref<ShortsConfig | null>(null);

    const pointA = computed(() => unref(abLoop.value?.pointA) ?? null);
    const pointB = computed(() => unref(abLoop.value?.pointB) ?? null);
    const length = computed(() =>
        pointA.value !== null && pointB.value !== null ? pointB.value - pointA.value : null,
    );
    const maxDuration = computed(() => config.value?.max_duration ?? 60);

    // The user may create shorts at all (scenes:upload).
    const canCreate = computed(() => config.value?.can_upload ?? false);

    const loadConfig = async () => {
        if (config.value) return;
        try {
            config.value = await getShortsConfig();
        } catch {
            config.value = null;
        }
    };

    const rangeLabel = computed(() => {
        if (pointA.value === null || pointB.value === null) return '';
        return `${formatShortClock(pointA.value)}-${formatShortClock(pointB.value)}`;
    });

    const validation = computed<{ valid: boolean; reason: string }>(() => {
        if (config.value && !config.value.can_create) {
            return {
                valid: false,
                reason: "Shorts can't be saved: the shorts folder can't be written to. An admin can fix this in Settings > Shorts",
            };
        }
        if (pointA.value === null) {
            return {
                valid: false,
                reason: 'Set a start point: move to where the short should begin and click A (or press O)',
            };
        }
        if (pointB.value === null) {
            return {
                valid: false,
                reason: 'Set an end point: move to where the short should end and click B (or press O)',
            };
        }
        if ((length.value ?? 0) < MIN_SHORT_SECONDS) {
            return { valid: false, reason: 'The short must be at least 1 second long' };
        }
        return {
            valid: true,
            reason: `Save ${rangeLabel.value} (${formatShortSpan(length.value ?? 0)}) as a short`,
        };
    });

    // Never disables saving: a long clip is kept but left out of the feed.
    const warning = computed<string | null>(() => {
        if (length.value === null || length.value <= maxDuration.value) return null;
        return `This clip is ${formatShortSpan(length.value)}, longer than the ${formatShortSpan(maxDuration.value)} Shorts limit. It will be saved, but it won't appear in the Shorts feed.`;
    });

    const saveState = computed<SaveShortState>(() => ({
        valid: validation.value.valid,
        reason: validation.value.reason,
        warning: validation.value.valid ? warning.value : null,
    }));

    const start = () => {
        active.value = true;
        loadConfig();
    };

    const cancel = () => {
        active.value = false;
        abLoop.value?.clear();
    };

    const save = () => {
        if (!validation.value.valid) return;
        modalOpen.value = true;
    };

    // After a short is queued: leave short mode and clear the range.
    const finish = (taskId: string) => {
        lastTaskId.value = taskId;
        modalOpen.value = false;
        cancel();
    };

    return {
        active,
        modalOpen,
        saving,
        lastTaskId,
        config,
        pointA,
        pointB,
        length,
        maxDuration,
        canCreate,
        rangeLabel,
        validation,
        warning,
        saveState,
        loadConfig,
        start,
        cancel,
        save,
        finish,
    };
};

export type ShortEditor = ReturnType<typeof useShortEditor>;
