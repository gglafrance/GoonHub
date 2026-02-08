<script setup lang="ts">
import type { StudioListItem } from '~/types/studio';

const props = defineProps<{
    visible: boolean;
    sceneIds?: number[];
    selectionCount?: number;
}>();

const emit = defineEmits<{
    close: [];
    complete: [];
}>();

const explorerStore = useExplorerStore();
const { fetchStudios } = useApiStudios();
const { bulkUpdateStudio } = useApiExplorer();

const studios = ref<StudioListItem[]>([]);
const selectedStudio = ref<string>('');
const customInput = ref('');
const loading = ref(false);
const loadingStudios = ref(false);
const initialLoad = ref(true);
const error = ref<string | null>(null);
const searchQuery = ref('');
const searchInputRef = ref<HTMLInputElement | null>(null);

let searchTimeout: ReturnType<typeof setTimeout> | null = null;

const showSkeleton = computed(() => loadingStudios.value && initialLoad.value);

const resolvedCount = computed(() => props.selectionCount ?? explorerStore.selectionCount);

const displayStudio = computed(() => selectedStudio.value || customInput.value);

const loadStudios = async () => {
    loadingStudios.value = true;
    error.value = null;
    try {
        const res = await fetchStudios(1, 50, searchQuery.value || undefined, 'name_asc');
        studios.value = res.data || [];
        initialLoad.value = false;
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to load studios';
    } finally {
        loadingStudios.value = false;
    }
};

const debouncedLoad = () => {
    if (searchTimeout) clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => loadStudios(), 300);
};

watch(searchQuery, debouncedLoad);

watch(
    () => props.visible,
    (open) => {
        if (open) {
            loadStudios();
            nextTick(() => searchInputRef.value?.focus());
        } else {
            searchQuery.value = '';
            selectedStudio.value = '';
            customInput.value = '';
            error.value = null;
            initialLoad.value = true;
            studios.value = [];
        }
    },
    { immediate: true },
);

function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') emit('close');
}

function selectStudio(name: string) {
    if (selectedStudio.value === name) {
        selectedStudio.value = '';
    } else {
        selectedStudio.value = name;
        customInput.value = '';
    }
}

function onCustomInput() {
    selectedStudio.value = '';
}

const handleSubmit = async () => {
    loading.value = true;
    error.value = null;

    try {
        await bulkUpdateStudio({
            scene_ids: props.sceneIds ?? explorerStore.getSelectedSceneIDs(),
            studio: displayStudio.value,
        });
        emit('complete');
        emit('close');
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to update studio';
    } finally {
        loading.value = false;
    }
};
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
                                    <Icon
                                        name="heroicons:building-office"
                                        size="13"
                                        class="text-lava"
                                    />
                                </div>
                                <div>
                                    <h2 class="text-sm font-semibold text-white">Edit Studio</h2>
                                    <p class="text-dim text-[10px] leading-tight">
                                        {{ resolvedCount }} scenes selected
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

                        <!-- Custom input -->
                        <div class="border-border shrink-0 border-b px-4 py-3">
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Studio Name
                            </label>
                            <input
                                v-model="customInput"
                                type="text"
                                placeholder="Type a custom studio name..."
                                class="border-border bg-surface focus:border-lava/40
                                    focus:ring-lava/10 w-full rounded-lg border px-3 py-2 text-xs
                                    text-white placeholder-white/30 transition-all focus:ring-1
                                    focus:outline-none"
                                @input="onCustomInput"
                            />
                            <p class="text-dim mt-1.5 text-[10px]">
                                {{
                                    displayStudio
                                        ? `Will set studio to "${displayStudio}"`
                                        : 'Leave empty to clear studio from selected scenes'
                                }}
                            </p>
                        </div>

                        <!-- Search bar -->
                        <div class="shrink-0 px-4 pt-3 pb-2">
                            <div class="relative">
                                <Icon
                                    name="heroicons:magnifying-glass"
                                    size="14"
                                    class="text-dim pointer-events-none absolute top-1/2 left-2.5
                                        -translate-y-1/2"
                                />
                                <input
                                    ref="searchInputRef"
                                    v-model="searchQuery"
                                    type="text"
                                    placeholder="Search existing studios..."
                                    class="border-border bg-surface focus:border-lava/40
                                        focus:ring-lava/10 w-full rounded-lg border py-2 pr-3 pl-8
                                        text-xs text-white placeholder-white/30 transition-all
                                        focus:ring-1 focus:outline-none"
                                />
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

                        <!-- Studio list -->
                        <div class="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
                            <!-- Loading skeleton -->
                            <div v-if="showSkeleton" class="space-y-0.5 px-2 py-1">
                                <div
                                    v-for="i in 6"
                                    :key="i"
                                    class="flex items-center gap-2.5 rounded-lg px-2 py-2"
                                >
                                    <div
                                        class="bg-surface h-8 w-8 shrink-0 animate-pulse rounded-lg"
                                    />
                                    <div class="flex-1 space-y-1.5">
                                        <div
                                            class="bg-surface h-3 animate-pulse rounded"
                                            :style="{ width: `${40 + Math.random() * 40}%` }"
                                        />
                                        <div class="bg-surface h-2 w-14 animate-pulse rounded" />
                                    </div>
                                </div>
                            </div>

                            <!-- Empty state -->
                            <div
                                v-else-if="studios.length === 0 && !loadingStudios"
                                class="flex flex-col items-center justify-center py-10"
                            >
                                <div
                                    class="bg-surface mb-3 flex h-10 w-10 items-center
                                        justify-center rounded-full"
                                >
                                    <Icon
                                        name="heroicons:building-office"
                                        size="18"
                                        class="text-dim"
                                    />
                                </div>
                                <p class="text-dim text-xs">
                                    {{
                                        searchQuery ? 'No matching studios' : 'No studios available'
                                    }}
                                </p>
                            </div>

                            <!-- Studio rows -->
                            <div
                                v-else
                                class="transition-opacity duration-150"
                                :class="loadingStudios ? 'pointer-events-none opacity-40' : ''"
                            >
                                <button
                                    v-for="s in studios"
                                    :key="s.id"
                                    class="group relative flex w-full items-center gap-2.5
                                        rounded-lg px-2 py-2 text-left transition-all"
                                    :class="
                                        selectedStudio === s.name ? 'bg-lava/4' : 'hover:bg-white/3'
                                    "
                                    @click="selectStudio(s.name)"
                                >
                                    <!-- Selection indicator bar -->
                                    <div
                                        class="absolute top-2 bottom-2 left-0 w-0.5 rounded-full
                                            transition-all"
                                        :class="
                                            selectedStudio === s.name
                                                ? 'bg-lava opacity-100'
                                                : 'bg-transparent opacity-0'
                                        "
                                    />

                                    <!-- Logo -->
                                    <div
                                        class="bg-surface flex h-8 w-8 shrink-0 items-center
                                            justify-center overflow-hidden rounded-lg ring-1
                                            transition-all"
                                        :class="
                                            selectedStudio === s.name
                                                ? 'ring-lava/40 ring-2'
                                                : 'ring-white/6'
                                        "
                                    >
                                        <img
                                            v-if="s.logo"
                                            :src="s.logo"
                                            :alt="s.name"
                                            class="h-full w-full object-contain p-0.5"
                                        />
                                        <Icon
                                            v-else
                                            name="heroicons:building-office"
                                            size="14"
                                            class="text-dim"
                                        />
                                    </div>

                                    <!-- Info -->
                                    <div class="min-w-0 flex-1">
                                        <p
                                            class="truncate text-xs font-medium transition-colors"
                                            :class="
                                                selectedStudio === s.name
                                                    ? 'text-white'
                                                    : 'text-white/80 group-hover:text-white'
                                            "
                                        >
                                            {{ s.name }}
                                        </p>
                                        <p class="text-dim text-[10px]">
                                            {{ s.scene_count || 0 }}
                                            {{ (s.scene_count || 0) === 1 ? 'scene' : 'scenes' }}
                                        </p>
                                    </div>

                                    <!-- Selection check -->
                                    <div
                                        class="flex h-5 w-5 shrink-0 items-center justify-center
                                            rounded-full transition-all"
                                        :class="
                                            selectedStudio === s.name
                                                ? 'bg-lava/15'
                                                : 'bg-transparent'
                                        "
                                    >
                                        <Icon
                                            v-if="selectedStudio === s.name"
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
                            class="border-border flex shrink-0 items-center justify-between border-t
                                px-4 py-3"
                        >
                            <span class="text-dim text-[11px]">
                                <template v-if="displayStudio">
                                    <span class="text-lava font-medium">{{ displayStudio }}</span>
                                </template>
                                <template v-else>No studio set</template>
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
