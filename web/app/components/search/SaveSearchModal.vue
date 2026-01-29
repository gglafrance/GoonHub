<script setup lang="ts">
import type { SavedSearch, SavedSearchFilters } from '~/types/saved_search';

const props = defineProps<{
    visible: boolean;
    filters: SavedSearchFilters;
}>();

const emit = defineEmits<{
    close: [];
    saved: [search: SavedSearch];
}>();

const { createSavedSearch } = useApiSavedSearches();

const name = ref('');
const loading = ref(false);
const error = ref('');

const filterSummary = computed(() => {
    const parts: string[] = [];
    const f = props.filters;

    if (f.query) parts.push(`Query: "${f.query}"`);
    if (f.match_type && f.match_type !== 'broad') parts.push(`Match: ${f.match_type}`);
    if (f.selected_tags?.length) parts.push(`${f.selected_tags.length} tag(s)`);
    if (f.selected_actors?.length) parts.push(`${f.selected_actors.length} actor(s)`);
    if (f.studio) parts.push(`Studio: ${f.studio}`);
    if (f.resolution) parts.push(`${f.resolution}`);
    if (f.min_duration || f.max_duration) {
        const min = f.min_duration ? `${Math.floor(f.min_duration / 60)}m` : '0m';
        const max = f.max_duration ? `${Math.floor(f.max_duration / 60)}m` : '';
        parts.push(`Duration: ${min}${max ? ` - ${max}` : '+'}`);
    }
    if (f.min_date || f.max_date) {
        parts.push(`Date: ${f.min_date || '...'} - ${f.max_date || '...'}`);
    }
    if (f.liked) parts.push('Liked only');
    if (f.min_rating || f.max_rating) {
        parts.push(`Rating: ${f.min_rating || 0} - ${f.max_rating || 5}`);
    }
    if (f.min_jizz_count || f.max_jizz_count) {
        parts.push(`Jizz: ${f.min_jizz_count || 0}+`);
    }
    if (f.sort) parts.push(`Sort: ${f.sort}`);

    return parts.length > 0 ? parts.join(', ') : 'No filters applied';
});

watch(
    () => props.visible,
    (visible) => {
        if (visible) {
            name.value = '';
            error.value = '';
        }
    },
);

const handleSubmit = async () => {
    if (!name.value.trim()) {
        error.value = 'Please enter a name for this search';
        return;
    }

    error.value = '';
    loading.value = true;

    try {
        const savedSearch = await createSavedSearch({
            name: name.value.trim(),
            filters: props.filters,
        });
        emit('saved', savedSearch);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save search';
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    error.value = '';
    emit('close');
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/70
                p-4 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div class="glass-panel border-border my-8 w-full max-w-md border p-6">
                <h3 class="mb-4 text-sm font-semibold text-white">Save Search</h3>

                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>

                <form @submit.prevent="handleSubmit" class="space-y-4">
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Name *
                        </label>
                        <input
                            v-model="name"
                            type="text"
                            required
                            maxlength="255"
                            placeholder="e.g., Favorite scenes, New releases..."
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none"
                        />
                    </div>

                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Filters to save
                        </label>
                        <div
                            class="text-dim bg-void/50 border-border rounded-lg border px-3 py-2
                                text-xs leading-relaxed"
                        >
                            {{ filterSummary }}
                        </div>
                    </div>

                    <div class="flex justify-end gap-2 pt-2">
                        <button
                            type="button"
                            @click="handleClose"
                            class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                                hover:text-white"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            :disabled="loading || !name.trim()"
                            class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                font-semibold text-white transition-all disabled:cursor-not-allowed
                                disabled:opacity-40"
                        >
                            {{ loading ? 'Saving...' : 'Save Search' }}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    </Teleport>
</template>
