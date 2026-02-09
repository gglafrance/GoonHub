<script setup lang="ts">
import type { ActorListItem } from '~/types/actor';

const props = withDefaults(
    defineProps<{
        visible: boolean;
        multiSelect?: boolean;
        showModeSelector?: boolean;
        excludeActorIds?: number[];
        sceneIds?: number[];
        selectionCount?: number;
        title?: string;
    }>(),
    {
        multiSelect: false,
        showModeSelector: false,
        excludeActorIds: () => [],
        sceneIds: () => [],
        selectionCount: 0,
        title: undefined,
    },
);

const emit = defineEmits<{
    close: [];
    select: [id: number];
    create: [name: string];
    complete: [];
}>();

const { fetchActors, createActor } = useApiActors();
const { bulkUpdateActors } = useApiExplorer();

const actors = ref<ActorListItem[]>([]);
const selectedActorIDs = ref<Set<number>>(new Set());
const mode = ref<'add' | 'remove' | 'replace'>('add');
const loading = ref(false);
const loadingActors = ref(false);
const initialLoad = ref(true);
const error = ref<string | null>(null);
const searchQuery = ref('');
const sortMode = ref<'az' | 'za' | 'most' | 'least'>('az');
const searchInputRef = ref<HTMLInputElement | null>(null);
const creating = ref(false);

// Show skeleton only on first load; subsequent fetches keep the list visible
const showSkeleton = computed(() => loadingActors.value && initialLoad.value);

let searchTimeout: ReturnType<typeof setTimeout> | null = null;

const sortOptions = [
    { key: 'az' as const, label: 'A-Z', icon: 'heroicons:bars-arrow-down' },
    { key: 'za' as const, label: 'Z-A', icon: 'heroicons:bars-arrow-up' },
    { key: 'most' as const, label: 'Most scenes', icon: 'heroicons:chart-bar' },
    { key: 'least' as const, label: 'Least scenes', icon: 'heroicons:chart-bar' },
] as const;

const sortApiMap: Record<string, string> = {
    az: 'name_asc',
    za: 'name_desc',
    most: 'scene_count_desc',
    least: 'scene_count_asc',
};

const modeOptions = [
    { key: 'add' as const, label: 'Add', icon: 'heroicons:plus', desc: 'Add to existing cast' },
    {
        key: 'remove' as const,
        label: 'Remove',
        icon: 'heroicons:minus',
        desc: 'Remove from videos',
    },
    {
        key: 'replace' as const,
        label: 'Replace',
        icon: 'heroicons:arrows-right-left',
        desc: 'Replace all actors',
    },
] as const;

const currentSort = computed(() => sortOptions.find((s) => s.key === sortMode.value)!);

function cycleSortMode() {
    const keys = sortOptions.map((s) => s.key);
    const idx = keys.indexOf(sortMode.value);
    sortMode.value = keys[(idx + 1) % keys.length] || 'az';
}

const modalTitle = computed(() => {
    if (props.title) return props.title;
    return props.multiSelect ? 'Edit Actors' : 'Add Actor';
});

const filteredActors = computed(() => {
    if (props.excludeActorIds.length === 0) return actors.value;
    const excludeSet = new Set(props.excludeActorIds);
    return actors.value.filter((a) => !excludeSet.has(a.id));
});

const maxSceneCount = computed(() => {
    if (filteredActors.value.length === 0) return 1;
    return Math.max(...filteredActors.value.map((a) => a.scene_count || 0), 1);
});

const showCreateOption = computed(
    () => searchQuery.value.trim() && !showSkeleton.value && !creating.value,
);

const loadActors = async () => {
    loadingActors.value = true;
    error.value = null;
    try {
        const res = await fetchActors(
            1,
            50,
            searchQuery.value || undefined,
            sortApiMap[sortMode.value],
        );
        actors.value = res.data || [];
        initialLoad.value = false;
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to load actors';
    } finally {
        loadingActors.value = false;
    }
};

const debouncedLoad = () => {
    if (searchTimeout) clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => loadActors(), 300);
};

watch(searchQuery, debouncedLoad);
watch(sortMode, () => loadActors());

watch(
    () => props.visible,
    (open) => {
        if (open) {
            loadActors();
            nextTick(() => searchInputRef.value?.focus());
        } else {
            searchQuery.value = '';
            selectedActorIDs.value = new Set();
            mode.value = 'add';
            error.value = null;
            initialLoad.value = true;
            actors.value = [];
        }
    },
    { immediate: true },
);

function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') emit('close');
}

function toggleActor(actorId: number) {
    if (selectedActorIDs.value.has(actorId)) {
        selectedActorIDs.value.delete(actorId);
    } else {
        selectedActorIDs.value.add(actorId);
    }
    selectedActorIDs.value = new Set(selectedActorIDs.value);
}

function handleActorClick(actorId: number) {
    if (props.multiSelect) {
        toggleActor(actorId);
    } else {
        emit('select', actorId);
    }
}

function handleCreateClick() {
    if (props.multiSelect) {
        handleCreateActor(searchQuery.value);
    } else {
        emit('create', searchQuery.value);
    }
}

async function handleCreateActor(name: string) {
    creating.value = true;
    try {
        const res = await createActor({ name });
        selectedActorIDs.value.add(res.id);
        selectedActorIDs.value = new Set(selectedActorIDs.value);
        searchQuery.value = '';
        await loadActors();
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to create actor';
    } finally {
        creating.value = false;
    }
}

async function handleSubmit() {
    if (selectedActorIDs.value.size === 0 && mode.value !== 'replace') {
        error.value = 'Select at least one actor';
        return;
    }

    loading.value = true;
    error.value = null;

    try {
        await bulkUpdateActors({
            scene_ids: props.sceneIds,
            actor_ids: Array.from(selectedActorIDs.value),
            mode: mode.value,
        });
        emit('complete');
        emit('close');
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to update actors';
    } finally {
        loading.value = false;
    }
}
</script>

<template>
    <Teleport to="body">
        <Transition
            enter-active-class="transition duration-200 ease-out"
            enter-from-class="opacity-0"
            enter-to-class="opacity-100"
            leave-active-class="transition duration-150 ease-in"
            leave-from-class="opacity-100"
            leave-to-class="opacity-0"
        >
            <div
                v-if="visible"
                class="fixed inset-0 z-50 flex items-center justify-center bg-black/60
                    backdrop-blur-sm"
                @click.self="$emit('close')"
                @keydown="onKeydown"
            >
                <Transition
                    enter-active-class="transition duration-200 ease-out"
                    enter-from-class="scale-95 opacity-0"
                    enter-to-class="scale-100 opacity-100"
                    leave-active-class="transition duration-150 ease-in"
                    leave-from-class="scale-100 opacity-100"
                    leave-to-class="scale-95 opacity-0"
                    appear
                >
                    <div
                        class="border-border bg-panel flex h-[50dvh] w-full max-w-md flex-col
                            rounded-xl border shadow-2xl"
                    >
                        <!-- Header -->
                        <div
                            class="border-border flex shrink-0 items-center justify-between border-b
                                px-4 py-3"
                        >
                            <div class="flex items-center gap-2.5">
                                <div
                                    class="bg-lava/10 flex h-6 w-6 items-center justify-center
                                        rounded-lg"
                                >
                                    <Icon name="heroicons:user-group" size="13" class="text-lava" />
                                </div>
                                <div>
                                    <h2 class="text-sm font-semibold text-white">
                                        {{ modalTitle }}
                                    </h2>
                                    <p
                                        v-if="multiSelect && showModeSelector && selectionCount"
                                        class="text-dim text-[10px] leading-tight"
                                    >
                                        {{ selectionCount }} scenes selected
                                    </p>
                                </div>
                            </div>
                            <button
                                class="text-dim flex items-center justify-center rounded-lg p-1.5
                                    transition-colors hover:bg-white/5 hover:text-white"
                                @click="$emit('close')"
                            >
                                <Icon name="heroicons:x-mark" size="16" />
                            </button>
                        </div>

                        <!-- Mode selector (multi-select bulk only) -->
                        <div
                            v-if="multiSelect && showModeSelector"
                            class="border-border shrink-0 border-b px-4 py-2.5"
                        >
                            <div class="bg-surface flex gap-0.5 rounded-lg p-0.5">
                                <button
                                    v-for="m in modeOptions"
                                    :key="m.key"
                                    class="flex flex-1 items-center justify-center gap-1.5
                                        rounded-md py-1.5 text-[11px] font-medium transition-all"
                                    :class="
                                        mode === m.key
                                            ? 'bg-lava/15 text-lava shadow-sm'
                                            : 'text-dim hover:text-white'
                                    "
                                    @click="mode = m.key"
                                >
                                    <Icon :name="m.icon" size="12" />
                                    {{ m.label }}
                                </button>
                            </div>
                            <p class="text-dim mt-1.5 px-0.5 text-[10px]">
                                {{ modeOptions.find((m) => m.key === mode)?.desc }}
                            </p>
                        </div>

                        <!-- Search bar -->
                        <div class="shrink-0 px-4 pt-3 pb-2">
                            <div class="flex items-center gap-2">
                                <div class="relative flex-1">
                                    <Icon
                                        name="heroicons:magnifying-glass"
                                        size="14"
                                        class="text-dim pointer-events-none absolute top-1/2
                                            left-2.5 -translate-y-1/2"
                                    />
                                    <input
                                        ref="searchInputRef"
                                        v-model="searchQuery"
                                        type="text"
                                        placeholder="Search actors..."
                                        class="border-border bg-surface focus:border-lava/40
                                            focus:ring-lava/10 w-full rounded-lg border py-2 pr-3
                                            pl-8 text-xs text-white placeholder-white/30
                                            transition-all focus:ring-1 focus:outline-none"
                                    />
                                </div>
                                <button
                                    class="border-border bg-surface hover:border-border-hover
                                        text-dim group flex shrink-0 items-center gap-1.5 rounded-lg
                                        border px-2 py-2 text-[10px] font-medium transition-all
                                        hover:text-white"
                                    :title="`Sort: ${currentSort.label}`"
                                    @click="cycleSortMode()"
                                >
                                    <Icon :name="currentSort.icon" size="12" />
                                    <span class="hidden min-[400px]:inline">
                                        {{ currentSort.label }}
                                    </span>
                                </button>
                            </div>
                        </div>

                        <!-- Error -->
                        <div v-if="error" class="shrink-0 px-4 pb-2">
                            <div
                                class="border-lava/20 bg-lava/5 flex items-center gap-2 rounded-lg
                                    border px-3 py-2"
                            >
                                <Icon
                                    name="heroicons:exclamation-triangle"
                                    size="13"
                                    class="text-lava shrink-0"
                                />
                                <span class="text-[11px] text-red-300">{{ error }}</span>
                            </div>
                        </div>

                        <!-- Actor list -->
                        <div class="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
                            <!-- Loading skeleton (initial load only) -->
                            <div v-if="showSkeleton" class="space-y-0.5 px-2 py-1">
                                <div
                                    v-for="i in 6"
                                    :key="i"
                                    class="flex items-center gap-2.5 rounded-lg px-2 py-2"
                                >
                                    <div
                                        class="bg-surface h-8 w-8 shrink-0 animate-pulse
                                            rounded-full"
                                    />
                                    <div class="flex-1 space-y-1.5">
                                        <div
                                            class="bg-surface h-3 animate-pulse rounded"
                                            :style="{ width: `${50 + Math.random() * 30}%` }"
                                        />
                                        <div class="bg-surface h-2 w-12 animate-pulse rounded" />
                                    </div>
                                </div>
                            </div>

                            <!-- Empty state -->
                            <div
                                v-else-if="
                                    filteredActors.length === 0 &&
                                    !showCreateOption &&
                                    !loadingActors
                                "
                                class="flex flex-col items-center justify-center py-10"
                            >
                                <div
                                    class="bg-surface mb-3 flex h-10 w-10 items-center
                                        justify-center rounded-full"
                                >
                                    <Icon name="heroicons:user-group" size="18" class="text-dim" />
                                </div>
                                <p class="text-dim text-xs">
                                    {{ searchQuery ? 'No matching actors' : 'No actors available' }}
                                </p>
                            </div>

                            <!-- Results -->
                            <div
                                v-else
                                class="transition-opacity duration-150"
                                :class="loadingActors ? 'pointer-events-none opacity-40' : ''"
                            >
                                <!-- Create new actor option -->
                                <button
                                    v-if="showCreateOption"
                                    class="group mb-0.5 flex w-full items-center gap-2.5 rounded-lg
                                        px-2 py-2 text-left transition-all hover:bg-white/3"
                                    @click="handleCreateClick"
                                >
                                    <div
                                        class="border-lava/30 bg-lava/5 group-hover:border-lava/50
                                            group-hover:bg-lava/10 flex h-8 w-8 shrink-0
                                            items-center justify-center rounded-full border
                                            border-dashed transition-colors"
                                    >
                                        <Icon name="heroicons:plus" size="14" class="text-lava" />
                                    </div>
                                    <div class="min-w-0 flex-1">
                                        <p class="text-lava truncate text-xs font-medium">
                                            Create "{{ searchQuery.trim() }}"
                                        </p>
                                        <p class="text-dim text-[10px]">Add new actor</p>
                                    </div>
                                </button>

                                <!-- Divider between create and list -->
                                <div
                                    v-if="showCreateOption && filteredActors.length > 0"
                                    class="border-border mx-2 mb-0.5 border-b"
                                />

                                <!-- Actor rows -->
                                <button
                                    v-for="actor in filteredActors"
                                    :key="actor.id"
                                    class="group relative flex w-full items-center gap-2.5
                                        rounded-lg px-2 py-2 text-left transition-all"
                                    :class="[
                                        selectedActorIDs.has(actor.id)
                                            ? 'bg-lava/4'
                                            : 'hover:bg-white/3',
                                    ]"
                                    @click="handleActorClick(actor.id)"
                                >
                                    <!-- Selection indicator bar -->
                                    <div
                                        v-if="multiSelect"
                                        class="absolute top-2 bottom-2 left-0 w-0.5 rounded-full
                                            transition-all"
                                        :class="
                                            selectedActorIDs.has(actor.id)
                                                ? 'bg-lava opacity-100'
                                                : 'bg-transparent opacity-0'
                                        "
                                    />

                                    <!-- Avatar -->
                                    <div
                                        class="relative h-8 w-8 shrink-0 overflow-hidden
                                            rounded-full transition-all"
                                        :class="
                                            selectedActorIDs.has(actor.id)
                                                ? 'ring-lava/40 ring-2'
                                                : 'ring-1 ring-white/6'
                                        "
                                    >
                                        <img
                                            v-if="actor.image_url"
                                            :src="actor.image_url"
                                            :alt="actor.name"
                                            class="h-full w-full object-cover"
                                        />
                                        <div
                                            v-else
                                            class="bg-surface flex h-full w-full items-center
                                                justify-center"
                                        >
                                            <Icon
                                                name="heroicons:user"
                                                size="14"
                                                class="text-dim"
                                            />
                                        </div>
                                    </div>

                                    <!-- Info -->
                                    <div class="min-w-0 flex-1">
                                        <p
                                            class="truncate text-xs font-medium transition-colors"
                                            :class="
                                                selectedActorIDs.has(actor.id)
                                                    ? 'text-white'
                                                    : 'text-white/80 group-hover:text-white'
                                            "
                                        >
                                            {{ actor.name }}
                                        </p>
                                        <div class="mt-0.5 flex items-center gap-2">
                                            <span class="text-dim text-[10px]">
                                                {{ actor.scene_count || 0 }}
                                                {{
                                                    (actor.scene_count || 0) === 1
                                                        ? 'scene'
                                                        : 'scenes'
                                                }}
                                            </span>
                                            <!-- Mini bar graph -->
                                            <div
                                                v-if="actor.scene_count"
                                                class="bg-surface h-1 w-12 overflow-hidden
                                                    rounded-full"
                                            >
                                                <div
                                                    class="h-full rounded-full transition-all"
                                                    :class="
                                                        selectedActorIDs.has(actor.id)
                                                            ? 'bg-lava/50'
                                                            : 'bg-white/15'
                                                    "
                                                    :style="{
                                                        width: `${Math.max(((actor.scene_count || 0) / maxSceneCount) * 100, 4)}%`,
                                                    }"
                                                />
                                            </div>
                                        </div>
                                    </div>

                                    <!-- Selection check -->
                                    <div
                                        v-if="multiSelect"
                                        class="flex h-5 w-5 shrink-0 items-center justify-center
                                            rounded-full transition-all"
                                        :class="
                                            selectedActorIDs.has(actor.id)
                                                ? 'bg-lava/15'
                                                : 'bg-transparent'
                                        "
                                    >
                                        <Icon
                                            v-if="selectedActorIDs.has(actor.id)"
                                            name="heroicons:check"
                                            size="12"
                                            class="text-lava"
                                        />
                                    </div>
                                </button>
                            </div>
                        </div>

                        <!-- Footer -->
                        <div
                            v-if="multiSelect"
                            class="border-border flex shrink-0 items-center justify-between border-t
                                px-4 py-3"
                        >
                            <span class="text-dim text-[11px]">
                                <template v-if="selectedActorIDs.size > 0">
                                    <span class="text-lava font-medium">
                                        {{ selectedActorIDs.size }}
                                    </span>
                                    selected
                                </template>
                                <template v-else>No actors selected</template>
                            </span>
                            <div class="flex items-center gap-2">
                                <button
                                    class="border-border hover:border-border-hover rounded-lg border
                                        px-3 py-1.5 text-xs font-medium text-white transition-all"
                                    @click="$emit('close')"
                                >
                                    Cancel
                                </button>
                                <button
                                    :disabled="loading"
                                    class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-xs
                                        font-semibold text-white transition-colors
                                        disabled:opacity-50"
                                    @click="handleSubmit"
                                >
                                    <span v-if="loading" class="flex items-center gap-1.5">
                                        <Icon name="svg-spinners:90-ring-with-bg" size="12" />
                                        Applying
                                    </span>
                                    <span v-else>Apply</span>
                                </button>
                            </div>
                        </div>
                    </div>
                </Transition>
            </div>
        </Transition>
    </Teleport>
</template>
