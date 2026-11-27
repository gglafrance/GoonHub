<script setup lang="ts">
import type { TriageSceneDetails } from '~/composables/useTriageSceneDetails';

// Triage mode: walk a queue of scenes one at a time and curate each with the keyboard.
definePageMeta({
    middleware: ['auth'],
});

useHead({ title: 'Triage' });
useSeoMeta({
    title: 'Triage',
    ogTitle: 'Triage - GoonHub',
    description: 'Curate your scenes one at a time',
});

const route = useRoute();
const router = useRouter();
const searchStore = useSearchStore();
const { fetchSavedSearch } = useApiSavedSearches();
const perms = useMyPermissions();
const queue = useTriageQueue();
const sceneDetails = useTriageSceneDetails();
const toasts = useTriageToasts();

const current = ref<TriageSceneDetails | null>(null);
const atEnd = ref(false);
const ready = ref(false);
const showShortcuts = ref(false);
const showQueueMobile = ref(false);

const media = ref<{ togglePlay: () => void } | null>(null);
const editors = ref<{
    focusTags: () => void;
    focusActors: () => void;
    focusStudio: () => void;
} | null>(null);

const advance = async () => {
    atEnd.value = !(await queue.next());
};

const goPrevious = async () => {
    if (await queue.previous()) atEnd.value = false;
};

const actions = useTriageActions({ current, details: sceneDetails, queue, toasts, advance });

const number = new Intl.NumberFormat();
const currentSkipped = computed(
    () => queue.currentId.value !== null && queue.skipped.value.has(queue.currentId.value),
);
const switching = computed(
    () => queue.currentId.value !== null && current.value?.id !== queue.currentId.value,
);
const progress = computed(() =>
    queue.total.value > 0 ? Math.min(100, (queue.position.value / queue.total.value) * 100) : 0,
);

// Queue source: the search store filters, mirrored in the URL
const openQueue = async (at: number, sceneId: number | null) => {
    atEnd.value = false;
    await queue.open(searchStore.getSearchParams(), at, sceneId);
};

const syncUrl = () => {
    const query = triageQueryFromParams(
        searchStore.getSearchParams(),
        queue.position.value,
        queue.currentId.value,
    );
    router.replace({ query });
};

const filterSignature = computed(() => triageQueueKey(searchStore.getSearchParams()));
let filterTimer: ReturnType<typeof setTimeout> | null = null;
watch(filterSignature, (sig) => {
    if (!ready.value || sig === queue.key.value) return;
    if (filterTimer) clearTimeout(filterTimer);
    filterTimer = setTimeout(() => void openQueue(1, null), 300);
});

// Shown scene
watch(
    () => queue.currentId.value,
    async (id) => {
        // The previous scene stays on screen (dimmed, actions off) until this one is loaded
        if (id === null) {
            current.value = null;
            return;
        }
        atEnd.value = false;
        const details = await sceneDetails.load(id);
        if (queue.currentId.value !== id) return;
        current.value = details;
        sceneDetails.prefetch(queue.upcoming(2));
    },
);

watch(
    () => [queue.position.value, queue.currentId.value],
    () => {
        if (ready.value) syncUrl();
    },
);

// Keyboard
const isTyping = (el: EventTarget | null) => {
    const t = el as HTMLElement | null;
    if (!t) return false;
    return (
        t.tagName === 'INPUT' ||
        t.tagName === 'TEXTAREA' ||
        t.tagName === 'SELECT' ||
        t.isContentEditable
    );
};

const onKeydown = (e: KeyboardEvent) => {
    if (showShortcuts.value || e.defaultPrevented) return;
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (isTyping(e.target)) return;
    const target = e.target as HTMLElement | null;
    // Enter and Space keep their meaning on a focused button or link
    const onControl = !!target && (target.tagName === 'BUTTON' || target.tagName === 'A');

    const handled = (fn: () => unknown) => {
        e.preventDefault();
        e.stopPropagation();
        void fn();
    };

    // Digits by physical key, so they work on AZERTY layouts too
    const digit = /^(?:Digit|Numpad)([0-5])$/.exec(e.code);
    if (digit) return handled(() => actions.rate(Number(digit[1])));

    switch (e.key) {
        case 'ArrowRight':
            return handled(advance);
        case ' ':
        case 'Enter':
            if (onControl) return;
            return handled(advance);
        case 'ArrowLeft':
            return handled(goPrevious);
        case 'Delete':
        case 'Backspace':
            if (!actions.canTrash.value) return;
            return handled(actions.trash);
        case '?':
            return handled(() => (showShortcuts.value = true));
    }

    switch (e.key.toLowerCase()) {
        case 'l':
            return handled(actions.toggleLike);
        case 'p':
            return handled(() => media.value?.togglePlay());
        case 't':
            if (actions.canEdit.value) handled(() => editors.value?.focusTags());
            return;
        case 'a':
            if (actions.canEdit.value) handled(() => editors.value?.focusActors());
            return;
        case 's':
            if (actions.canEdit.value) handled(() => editors.value?.focusStudio());
            return;
    }
};

const onBeforeUnload = () => actions.flushPendingTrash();

onMounted(async () => {
    // Capture phase: the triage keys win over the video element's own keys
    window.addEventListener('keydown', onKeydown, true);
    window.addEventListener('beforeunload', onBeforeUnload);
    void perms.load();
    searchStore.loadFilterOptions();

    const savedUuid = str(route.query.saved);
    let loaded = false;
    if (savedUuid) {
        try {
            const saved = await fetchSavedSearch(savedUuid);
            searchStore.loadFilters(saved.filters);
            loaded = true;
        } catch {
            toasts.error('Saved search not found');
        }
    }
    if (!loaded) {
        if (hasTriageFilters(route.query)) {
            searchStore.loadFilters(triageFiltersFromQuery(route.query));
            const seed = triageSeedFromQuery(route.query);
            if (seed) searchStore.seed = seed;
        } else {
            // Default queue: scenes without tags
            searchStore.resetFilters();
            searchStore.status.has_tags = false;
        }
    }

    const at = loaded ? 1 : Number(str(route.query.i)) || 1;
    const sceneId = loaded ? null : Number(str(route.query.scene)) || null;
    await openQueue(at, sceneId);
    ready.value = true;
    syncUrl();
});

onBeforeUnmount(() => {
    window.removeEventListener('keydown', onKeydown, true);
    window.removeEventListener('beforeunload', onBeforeUnload);
    if (filterTimer) clearTimeout(filterTimer);
    actions.flushPendingTrash();
});

function str(v: unknown): string {
    return Array.isArray(v) ? String(v[0] ?? '') : typeof v === 'string' ? v : '';
}
</script>

<template>
    <div class="mx-auto max-w-415 px-4 py-4 sm:py-6">
        <div class="flex gap-5">
            <aside class="hidden w-60 shrink-0 lg:block">
                <TriageQueuePanel />
            </aside>

            <div class="min-w-0 flex-1 space-y-3">
                <!-- Header: progress and navigation -->
                <div class="flex items-center gap-3">
                    <div class="flex items-center gap-2">
                        <Icon name="heroicons:bolt" size="16" class="text-lava" />
                        <h1 class="text-sm font-semibold text-white">Triage</h1>
                    </div>
                    <div class="text-muted font-mono text-xs">
                        <template v-if="queue.total.value > 0">
                            {{ number.format(queue.position.value) }}
                            <span class="text-dim">/ {{ number.format(queue.total.value) }}</span>
                        </template>
                        <span v-else-if="queue.loading.value" class="text-dim">Loading...</span>
                    </div>

                    <div class="ml-auto flex items-center gap-1.5">
                        <button
                            class="border-border text-dim hover:border-lava/30 hover:text-lava flex
                                h-8 items-center gap-1 rounded-md border px-2 text-[11px]
                                transition-all lg:hidden"
                            @click="showQueueMobile = !showQueueMobile"
                        >
                            <Icon name="heroicons:adjustments-horizontal" size="14" />
                            Queue
                        </button>
                        <button
                            class="border-border text-dim hover:border-lava/30 hover:text-lava flex
                                h-8 w-8 items-center justify-center rounded-md border transition-all
                                disabled:opacity-40"
                            title="Previous (Left)"
                            :disabled="queue.position.value <= 1"
                            @click="goPrevious"
                        >
                            <Icon name="heroicons:chevron-left" size="16" />
                        </button>
                        <button
                            class="border-border text-dim hover:border-lava/30 hover:text-lava flex
                                h-8 items-center gap-1 rounded-md border px-2.5 text-[11px]
                                transition-all disabled:opacity-40"
                            title="Next (Right, Space, Enter)"
                            :disabled="queue.ids.value.length === 0"
                            @click="advance"
                        >
                            Next
                            <Icon name="heroicons:chevron-right" size="14" />
                        </button>
                        <button
                            class="border-border text-dim hover:border-lava/30 hover:text-lava
                                hidden h-8 w-8 items-center justify-center rounded-md border
                                transition-all sm:flex"
                            title="Keyboard shortcuts (?)"
                            @click="showShortcuts = true"
                        >
                            <Icon name="heroicons:command-line" size="16" />
                        </button>
                    </div>
                </div>

                <div class="h-0.5 overflow-hidden rounded-full bg-white/8">
                    <div
                        class="bg-lava h-full rounded-full transition-all duration-300"
                        :style="{ width: `${progress}%` }"
                    />
                </div>

                <div
                    v-if="showQueueMobile"
                    class="border-border bg-surface/60 rounded-xl border p-4 lg:hidden"
                >
                    <TriageQueuePanel />
                </div>

                <ErrorAlert v-if="queue.error.value" :message="queue.error.value" />

                <!-- Empty queue -->
                <div
                    v-if="queue.isEmpty.value"
                    class="border-border flex flex-col items-center gap-2 rounded-xl border
                        bg-white/2 py-16 text-center"
                >
                    <Icon name="heroicons:check-circle" size="28" class="text-emerald" />
                    <p class="text-sm font-medium text-white">Nothing to triage here</p>
                    <p class="text-dim text-xs">
                        No scene matches this queue. Pick another preset or change the filters.
                    </p>
                </div>

                <template v-else>
                    <div
                        v-if="atEnd"
                        class="border-emerald/30 bg-emerald/5 text-emerald flex items-center gap-2
                            rounded-lg border px-3 py-2 text-xs"
                    >
                        <Icon name="heroicons:flag" size="14" />
                        End of the queue.
                        <button
                            class="hover:text-emerald/80 ml-auto font-medium underline-offset-2
                                hover:underline"
                            title="Reload the queue: the scenes that left it are no longer in it"
                            @click="openQueue(1, null)"
                        >
                            Start over
                        </button>
                    </div>

                    <div
                        class="grid gap-4 transition-opacity xl:grid-cols-[minmax(0,1fr)_320px]"
                        :class="{ 'pointer-events-none opacity-60': switching && current }"
                    >
                        <div class="min-w-0 space-y-3">
                            <div
                                v-if="currentSkipped"
                                class="border-border text-dim flex aspect-video w-full flex-col
                                    items-center justify-center gap-2 rounded-xl border bg-black
                                    text-xs"
                            >
                                <Icon name="heroicons:trash" size="24" />
                                Moved to trash
                            </div>
                            <TriageMedia
                                v-else
                                ref="media"
                                :scene="current?.scene ?? null"
                                :loading="queue.loading.value || queue.currentId.value !== null"
                            />

                            <TriageSceneInfo
                                v-if="current?.scene && !currentSkipped"
                                :scene="current.scene"
                                :rating="current.rating"
                                :liked="current.liked"
                                @rate="actions.rate"
                                @like="actions.toggleLike"
                            />
                            <div
                                v-else-if="current?.missing && !currentSkipped"
                                class="text-dim text-xs"
                            >
                                This scene is no longer available.
                            </div>

                            <TriageKeyHints
                                class="border-border border-t pt-3"
                                @help="showShortcuts = true"
                            />
                        </div>

                        <TriageEditors
                            v-if="current?.scene && !currentSkipped"
                            ref="editors"
                            :details="current"
                            :actions="actions"
                        />
                    </div>
                </template>
            </div>
        </div>

        <TriageToasts
            :toasts="toasts.toasts.value"
            @action="toasts.runAction"
            @dismiss="toasts.dismiss"
        />
        <KeyboardShortcutsModal
            :visible="showShortcuts"
            triage-first
            @close="showShortcuts = false"
        />
    </div>
</template>
