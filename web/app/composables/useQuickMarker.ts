import type videojs from 'video.js';
import type { Marker } from '~/types/marker';
import type { Tag } from '~/types/tag';
import type {
    QuickMarkerChip,
    QuickMarkerLabelOption,
    QuickMarkerTagOption,
} from '~/utils/quickMarker';

type Player = ReturnType<typeof videojs>;

export type QuickMarkerStep = 'label' | 'tags';

interface Options {
    player: () => Player | null | undefined;
    sceneId: () => number;
    // Called once the marker is saved (with its sub-labels, or when the dialog is closed after
    // the marker was saved but its sub-labels failed). Not refetched: the marker is returned
    // with its tags, its thumbnail comes later
    onSaved: (marker: Marker, label: string, time: number) => void;
}

const readLastLabel = (): string => {
    if (!import.meta.client) return '';
    try {
        return sessionStorage.getItem(QUICK_MARKER_LAST_LABEL_KEY) ?? '';
    } catch {
        return '';
    }
};

const writeLastLabel = (label: string) => {
    try {
        sessionStorage.setItem(QUICK_MARKER_LAST_LABEL_KEY, label);
    } catch {
        // Storage unavailable (private mode): the default label only lasts for this page
    }
};

const errorMessage = (e: unknown, fallback: string) =>
    e instanceof Error && e.message ? e.message : fallback;

/**
 * Quick marker of the watch page: M captures the current time and pauses, then asks for the
 * label, then for the sub-labels (marker tags), and saves the marker. Errors stay in the dialog
 * with what was typed; a marker already saved is never created twice on retry.
 */
export const useQuickMarker = ({ player, sceneId, onSaved }: Options) => {
    const { createMarker, fetchLabelSuggestions, addMarkerTags } = useApiMarkers();
    const { fetchTags, createTag } = useApiTags();
    const perms = useMyPermissions();

    const isOpen = ref(false);
    const step = ref<QuickMarkerStep>('label');
    const time = ref(0);
    const label = ref('');
    const lastLabel = ref('');
    const chips = ref<QuickMarkerChip[]>([]);
    const saving = ref(false);
    const error = ref<string | null>(null);
    // Marker saved by a previous attempt whose sub-labels failed
    const savedMarker = shallowRef<Marker | null>(null);
    const savedLabel = ref('');

    const labels = ref<QuickMarkerLabelOption[]>([]);
    const tags = ref<QuickMarkerTagOption[]>([]);
    let labelsLoaded = false;
    let tagsLoaded = false;
    let wasPlaying = false;

    // Same rule as the triage page: tags are created by the users who may edit scenes
    const canCreateTags = computed(() => perms.has('scenes:upload'));

    const loadLabels = async () => {
        if (labelsLoaded) return;
        try {
            const data = await fetchLabelSuggestions();
            labels.value = data.labels || [];
            labelsLoaded = true;
        } catch {
            // Autocomplete only: the label can still be typed
        }
    };

    const loadTags = async (force = false) => {
        if (tagsLoaded && !force) return;
        try {
            const res = await fetchTags();
            tags.value = (res.data || []) as Tag[];
            tagsLoaded = true;
        } catch {
            // Autocomplete only
        }
    };

    const resumePlayback = () => {
        if (!wasPlaying) return;
        wasPlaying = false;
        player()
            ?.play()
            ?.catch(() => {});
    };

    const reset = () => {
        isOpen.value = false;
        step.value = 'label';
        label.value = '';
        chips.value = [];
        error.value = null;
        saving.value = false;
        savedMarker.value = null;
        savedLabel.value = '';
    };

    /** Opens the dialog at the current player time (pauses a playing scene). */
    const open = () => {
        const p = player();
        if (!p || isOpen.value) return false;
        // currentTime() is the scene time, the transcode middleware adds its stream offset
        time.value = quickMarkerTimestamp(p.currentTime());
        wasPlaying = !p.paused();
        if (wasPlaying) p.pause();
        reset();
        lastLabel.value = readLastLabel();
        isOpen.value = true;
        void loadLabels();
        void loadTags();
        void perms.load();
        return true;
    };

    const close = () => {
        const marker = savedMarker.value;
        const name = savedLabel.value;
        reset();
        // The marker exists even though its sub-labels failed: show it
        if (marker) onSaved(marker, name, marker.timestamp);
        resumePlayback();
    };

    /** Closes without resuming playback nor refreshing (the scene changed). */
    const abandon = () => {
        wasPlaying = false;
        reset();
    };

    /** Enter on the label step. */
    const confirmLabel = () => {
        if (savedMarker.value) {
            step.value = 'tags';
            return;
        }
        const resolved = resolveQuickMarkerLabel(label.value, lastLabel.value);
        if (!resolved) {
            error.value = 'Type a label';
            return;
        }
        label.value = resolved;
        error.value = null;
        step.value = 'tags';
    };

    /** Esc on an empty sub-label input: back to the label (kept). */
    const backToLabel = () => {
        error.value = null;
        step.value = 'label';
    };

    const addChip = (chip: QuickMarkerChip) => {
        chips.value = addQuickMarkerChip(chips.value, chip);
    };

    const removeLastChip = () => {
        chips.value = removeLastQuickMarkerChip(chips.value);
    };

    const removeChip = (index: number) => {
        chips.value = chips.value.filter((_, i) => i !== index);
    };

    // Creates a new sub-label tag; a tag created meanwhile with the same name is reused
    const ensureTag = async (chip: QuickMarkerChip): Promise<QuickMarkerChip> => {
        if (chip.id != null) return chip;
        let tag: Tag | undefined;
        try {
            tag = await createTag(chip.name, QUICK_MARKER_NEW_TAG_COLOR);
        } catch (e) {
            await loadTags(true);
            const existing = findQuickMarkerTag(tags.value, chip.name);
            if (!existing) throw e;
            return { id: existing.id, name: existing.name, color: existing.color };
        }
        tags.value = [...tags.value, { ...tag!, scene_count: 0 }];
        return { id: tag!.id, name: tag!.name, color: tag!.color };
    };

    const rememberLabel = (name: string) => {
        lastLabel.value = name;
        writeLastLabel(name);
        const known = labels.value.find((l) => l.label === name);
        labels.value = known
            ? labels.value.map((l) => (l === known ? { ...l, count: l.count + 1 } : l))
            : [...labels.value, { label: name, count: 1 }];
    };

    /** Enter on an empty sub-label input: saves the marker, then its sub-labels. */
    const submit = async () => {
        if (saving.value || !isOpen.value) return;
        saving.value = true;
        error.value = null;
        try {
            let marker = savedMarker.value;
            if (!marker) {
                const name = resolveQuickMarkerLabel(label.value, lastLabel.value);
                if (!name) {
                    step.value = 'label';
                    error.value = 'Type a label';
                    return;
                }
                marker = await createMarker(sceneId(), { timestamp: time.value, label: name });
                savedMarker.value = marker;
                savedLabel.value = name;
                rememberLabel(name);
            }
            // New tags are created one by one; each created chip keeps its id for a retry
            for (let i = 0; i < chips.value.length; i++) {
                const chip = chips.value[i]!;
                if (chip.id != null) continue;
                const created = await ensureTag(chip);
                chips.value = chips.value.map((c, j) => (j === i ? created : c));
            }
            const ids = chips.value.map((c) => c.id).filter((id): id is number => id != null);
            if (ids.length) {
                const markerTags = await addMarkerTags(marker.id, ids);
                savedMarker.value = { ...marker, tags: markerTags };
            }
            close();
        } catch (e) {
            error.value = savedMarker.value
                ? `Marker saved, but not its sub-labels: ${errorMessage(e, 'request failed')}`
                : errorMessage(e, 'Failed to add the marker');
        } finally {
            saving.value = false;
        }
    };

    return {
        isOpen,
        step,
        time,
        label,
        lastLabel,
        chips,
        saving,
        error,
        labels,
        tags,
        canCreateTags,
        labelLocked: computed(() => savedMarker.value !== null),
        open,
        close,
        abandon,
        confirmLabel,
        backToLabel,
        addChip,
        removeChip,
        removeLastChip,
        submit,
    };
};

export type QuickMarkerState = ReturnType<typeof useQuickMarker>;
