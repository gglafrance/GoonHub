<script setup lang="ts">
import type { Tag } from '~/types/tag';

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
const { fetchTags } = useApiTags();
const { bulkUpdateTags } = useApiExplorer();

const allTags = ref<Tag[]>([]);
const selectedTagIDs = ref<Set<number>>(new Set());
const mode = ref<'add' | 'remove' | 'replace'>('add');
const loading = ref(false);
const loadingTags = ref(false);
const error = ref<string | null>(null);
const searchQuery = ref('');
const searchInputRef = ref<HTMLInputElement | null>(null);

const modeOptions = [
    { key: 'add' as const, label: 'Add', icon: 'heroicons:plus', desc: 'Add to existing tags' },
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
        desc: 'Replace all tags',
    },
] as const;

const resolvedCount = computed(() => props.selectionCount ?? explorerStore.selectionCount);

const filteredTags = computed(() => {
    if (!searchQuery.value.trim()) return allTags.value;
    const q = searchQuery.value.toLowerCase();
    return allTags.value.filter((t) => t.name.toLowerCase().includes(q));
});

watch(
    () => props.visible,
    async (open) => {
        if (open) {
            loadingTags.value = true;
            error.value = null;
            try {
                const res = await fetchTags();
                allTags.value = res.data || [];
            } catch (err) {
                error.value = err instanceof Error ? err.message : 'Failed to load tags';
            } finally {
                loadingTags.value = false;
            }
            nextTick(() => searchInputRef.value?.focus());
        } else {
            searchQuery.value = '';
            selectedTagIDs.value = new Set();
            mode.value = 'add';
            error.value = null;
        }
    },
    { immediate: true },
);

function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') emit('close');
}

const toggleTag = (tagId: number) => {
    if (selectedTagIDs.value.has(tagId)) {
        selectedTagIDs.value.delete(tagId);
    } else {
        selectedTagIDs.value.add(tagId);
    }
    selectedTagIDs.value = new Set(selectedTagIDs.value);
};

const handleSubmit = async () => {
    if (selectedTagIDs.value.size === 0 && mode.value !== 'replace') {
        error.value = 'Select at least one tag';
        return;
    }

    loading.value = true;
    error.value = null;

    try {
        await bulkUpdateTags({
            scene_ids: props.sceneIds ?? explorerStore.getSelectedSceneIDs(),
            tag_ids: Array.from(selectedTagIDs.value),
            mode: mode.value,
        });
        emit('complete');
        emit('close');
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to update tags';
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
                        class="border-border bg-panel flex w-full max-w-md flex-col rounded-xl
                            border shadow-2xl"
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
                                    <Icon name="heroicons:tag" size="13" class="text-lava" />
                                </div>
                                <div>
                                    <h2 class="text-sm font-semibold text-white">Edit Tags</h2>
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

                        <!-- Mode selector -->
                        <div class="border-border shrink-0 border-b px-4 py-2.5">
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
                                    placeholder="Search tags..."
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

                        <!-- Tag list -->
                        <div class="min-h-0 flex-1 overflow-y-auto px-4 pb-2">
                            <!-- Loading skeleton -->
                            <div v-if="loadingTags" class="flex flex-wrap gap-1.5 py-2">
                                <div
                                    v-for="i in 12"
                                    :key="i"
                                    class="bg-surface h-7 animate-pulse rounded-full"
                                    :style="{ width: `${50 + Math.random() * 50}px` }"
                                />
                            </div>

                            <!-- Empty state -->
                            <div
                                v-else-if="filteredTags.length === 0"
                                class="flex flex-col items-center justify-center py-10"
                            >
                                <div
                                    class="bg-surface mb-3 flex h-10 w-10 items-center
                                        justify-center rounded-full"
                                >
                                    <Icon name="heroicons:tag" size="18" class="text-dim" />
                                </div>
                                <p class="text-dim text-xs">
                                    {{ searchQuery ? 'No matching tags' : 'No tags available' }}
                                </p>
                            </div>

                            <!-- Tag pills -->
                            <div
                                v-else
                                class="flex max-h-64 flex-wrap gap-1.5 overflow-y-auto py-1"
                            >
                                <button
                                    v-for="tag in filteredTags"
                                    :key="tag.id"
                                    class="group flex items-center gap-1.5 rounded-full border
                                        px-2.5 py-1 text-[11px] font-medium transition-all"
                                    :class="
                                        selectedTagIDs.has(tag.id)
                                            ? 'ring-2'
                                            : 'opacity-60 hover:opacity-100'
                                    "
                                    :style="{
                                        borderColor: tag.color + '60',
                                        backgroundColor: selectedTagIDs.has(tag.id)
                                            ? tag.color + '20'
                                            : tag.color + '08',
                                        color: 'white',
                                        '--tw-ring-color': tag.color,
                                    }"
                                    @click="toggleTag(tag.id)"
                                >
                                    <span
                                        class="inline-block h-2 w-2 shrink-0 rounded-full
                                            transition-transform"
                                        :class="
                                            selectedTagIDs.has(tag.id) ? 'scale-110' : 'scale-100'
                                        "
                                        :style="{ backgroundColor: tag.color }"
                                    />
                                    {{ tag.name }}
                                    <span v-if="tag.scene_count" class="text-[9px] opacity-40">
                                        {{ tag.scene_count }}
                                    </span>
                                    <Icon
                                        v-if="selectedTagIDs.has(tag.id)"
                                        name="heroicons:check"
                                        size="10"
                                    />
                                </button>
                            </div>
                        </div>

                        <!-- Footer -->
                        <div
                            class="border-border flex shrink-0 items-center justify-between border-t
                                px-4 py-3"
                        >
                            <span class="text-dim text-[11px]">
                                <template v-if="selectedTagIDs.size > 0">
                                    <span class="text-lava font-medium">
                                        {{ selectedTagIDs.size }}
                                    </span>
                                    selected
                                </template>
                                <template v-else>No tags selected</template>
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
