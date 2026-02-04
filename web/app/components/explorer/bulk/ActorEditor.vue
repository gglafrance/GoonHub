<script setup lang="ts">
import type { ActorListItem } from '~/types/actor';

defineProps<{
    visible: boolean;
}>();

const emit = defineEmits<{
    close: [];
    complete: [];
}>();

const explorerStore = useExplorerStore();
const { fetchActors } = useApiActors();
const { bulkUpdateActors } = useApiExplorer();

const actors = ref<ActorListItem[]>([]);
const selectedActorIDs = ref<Set<number>>(new Set());
const mode = ref<'add' | 'remove' | 'replace'>('add');
const loading = ref(false);
const loadingActors = ref(false);
const error = ref<string | null>(null);
const searchQuery = ref('');

let searchTimeout: ReturnType<typeof setTimeout> | null = null;

onMounted(() => {
    loadActors();
});

const loadActors = async (query = '') => {
    loadingActors.value = true;
    try {
        const res = await fetchActors(1, 50, query);
        actors.value = res.data || [];
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to load actors';
    } finally {
        loadingActors.value = false;
    }
};

watch(searchQuery, () => {
    if (searchTimeout) {
        clearTimeout(searchTimeout);
    }
    searchTimeout = setTimeout(() => {
        loadActors(searchQuery.value);
    }, 300);
});

const toggleActor = (actorId: number) => {
    if (selectedActorIDs.value.has(actorId)) {
        selectedActorIDs.value.delete(actorId);
    } else {
        selectedActorIDs.value.add(actorId);
    }
    selectedActorIDs.value = new Set(selectedActorIDs.value);
};

const isActorSelected = (actorId: number) => selectedActorIDs.value.has(actorId);

const handleSubmit = async () => {
    if (selectedActorIDs.value.size === 0 && mode.value !== 'replace') {
        error.value = 'Select at least one actor';
        return;
    }

    loading.value = true;
    error.value = null;

    try {
        await bulkUpdateActors({
            scene_ids: explorerStore.getSelectedSceneIDs(),
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
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
            @click.self="$emit('close')"
        >
            <div class="border-border bg-panel w-full max-w-md rounded-xl border shadow-2xl">
                <!-- Header -->
                <div class="border-border flex items-center justify-between border-b px-4 py-3">
                    <h2 class="text-sm font-semibold text-white">Bulk Edit Actors</h2>
                    <button
                        class="text-dim transition-colors hover:text-white"
                        @click="$emit('close')"
                    >
                        <Icon name="heroicons:x-mark" size="18" />
                    </button>
                </div>

                <!-- Content -->
                <div class="p-4">
                    <p class="text-dim mb-4 text-xs">
                        Editing actors for {{ explorerStore.selectionCount }} scenes
                    </p>

                    <!-- Mode Selection -->
                    <div class="mb-4">
                        <label
                            class="text-dim mb-2 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Mode
                        </label>
                        <div class="flex gap-2">
                            <button
                                v-for="m in ['add', 'remove', 'replace'] as const"
                                :key="m"
                                class="rounded-lg border px-3 py-1.5 text-xs font-medium
                                    transition-all"
                                :class="
                                    mode === m
                                        ? 'border-lava bg-lava/10 text-lava'
                                        : `border-border hover:border-border-hover text-dim
                                            hover:text-white`
                                "
                                @click="mode = m"
                            >
                                {{ m.charAt(0).toUpperCase() + m.slice(1) }}
                            </button>
                        </div>
                        <p class="text-dim mt-1.5 text-[10px]">
                            <template v-if="mode === 'add'"
                                >Add selected actors to existing cast</template
                            >
                            <template v-else-if="mode === 'remove'"
                                >Remove selected actors from videos</template
                            >
                            <template v-else>Replace all actors with selected actors</template>
                        </p>
                    </div>

                    <!-- Error -->
                    <ErrorAlert v-if="error" :message="error" class="mb-4" />

                    <!-- Search -->
                    <div class="mb-3">
                        <div class="relative">
                            <Icon
                                name="heroicons:magnifying-glass"
                                size="14"
                                class="text-dim absolute top-1/2 left-2.5 -translate-y-1/2"
                            />
                            <input
                                v-model="searchQuery"
                                type="text"
                                placeholder="Search actors..."
                                class="border-border bg-surface focus:border-lava/50
                                    focus:ring-lava/20 w-full rounded-lg border py-2 pr-3 pl-8
                                    text-xs text-white placeholder-white/40 transition-all
                                    focus:ring-2 focus:outline-none"
                            />
                        </div>
                    </div>

                    <!-- Actor Selection -->
                    <div class="mb-4">
                        <div v-if="loadingActors" class="flex items-center justify-center py-4">
                            <LoadingSpinner />
                        </div>

                        <div
                            v-else-if="actors.length === 0"
                            class="text-dim py-4 text-center text-xs"
                        >
                            No actors found
                        </div>

                        <div v-else class="max-h-64 overflow-y-auto">
                            <div class="space-y-1">
                                <button
                                    v-for="actor in actors"
                                    :key="actor.id"
                                    class="border-border hover:border-border-hover flex w-full
                                        items-center gap-2 rounded-lg border p-2 text-left
                                        transition-all"
                                    :class="
                                        isActorSelected(actor.id)
                                            ? 'ring-lava/50 border-lava/30 ring-2'
                                            : ''
                                    "
                                    @click="toggleActor(actor.id)"
                                >
                                    <div
                                        class="bg-panel border-border flex h-8 w-8 shrink-0
                                            items-center justify-center overflow-hidden rounded-full
                                            border"
                                    >
                                        <img
                                            v-if="actor.image_url"
                                            :src="actor.image_url"
                                            :alt="actor.name"
                                            class="h-full w-full object-cover"
                                        />
                                        <Icon
                                            v-else
                                            name="heroicons:user"
                                            size="16"
                                            class="text-dim"
                                        />
                                    </div>
                                    <div class="min-w-0 flex-1">
                                        <p class="truncate text-xs font-medium text-white">
                                            {{ actor.name }}
                                        </p>
                                        <p class="text-dim text-[10px]">
                                            {{ actor.scene_count }} scenes
                                        </p>
                                    </div>
                                    <Icon
                                        v-if="isActorSelected(actor.id)"
                                        name="heroicons:check-circle"
                                        size="18"
                                        class="text-lava shrink-0"
                                    />
                                </button>
                            </div>
                        </div>
                    </div>

                    <!-- Selected Count -->
                    <div v-if="selectedActorIDs.size > 0" class="text-dim text-xs">
                        {{ selectedActorIDs.size }} actor(s) selected
                    </div>
                </div>

                <!-- Footer -->
                <div class="border-border flex items-center justify-end gap-2 border-t px-4 py-3">
                    <button
                        class="border-border hover:border-border-hover rounded-lg border px-3 py-1.5
                            text-xs font-medium text-white transition-all"
                        @click="$emit('close')"
                    >
                        Cancel
                    </button>
                    <button
                        :disabled="loading"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-xs
                            font-semibold text-white transition-colors disabled:opacity-50"
                        @click="handleSubmit"
                    >
                        <span v-if="loading">Applying...</span>
                        <span v-else>Apply</span>
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
