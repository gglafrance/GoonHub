<script setup lang="ts">
// Search (debounced), sort, order and the filters panel toggle of the trash list.
import type { TrashSort } from '~/types/trash';

const props = defineProps<{
    query: string;
    sort: TrashSort;
    order: 'asc' | 'desc';
    filtersOpen: boolean;
    filterCount: number;
}>();

const emit = defineEmits<{
    'update:query': [value: string];
    'update:sort': [value: TrashSort];
    toggleOrder: [];
    toggleFilters: [];
    refresh: [];
}>();

const text = ref(props.query);
watch(
    () => props.query,
    (v) => {
        if (v !== text.value.trim()) text.value = v;
    },
);

let timer: ReturnType<typeof setTimeout> | undefined;
watch(text, (v) => {
    clearTimeout(timer);
    timer = setTimeout(() => {
        if (v.trim() !== props.query) emit('update:query', v.trim());
    }, 300);
});
onBeforeUnmount(() => clearTimeout(timer));
</script>

<template>
    <div class="flex flex-wrap items-center gap-2">
        <div class="relative min-w-48 flex-1">
            <Icon
                name="heroicons:magnifying-glass"
                size="14"
                class="text-dim pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2"
            />
            <input
                v-model="text"
                type="search"
                placeholder="Search title or file path"
                class="border-border bg-surface w-full rounded-md border py-1.5 pr-2 pl-8 text-xs
                    text-white placeholder:text-white/30 focus:border-white/20 focus:outline-none"
            />
        </div>
        <button
            class="flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-[11px] font-medium
                transition-all"
            :class="
                filtersOpen || filterCount
                    ? 'border-lava/40 text-lava bg-lava/5'
                    : 'border-border text-dim hover:text-white'
            "
            @click="emit('toggleFilters')"
        >
            <Icon name="heroicons:funnel" size="13" />
            Filters
            <span
                v-if="filterCount"
                class="bg-lava/20 text-lava rounded-full px-1.5 text-[10px] font-semibold"
            >
                {{ filterCount }}
            </span>
        </button>
        <div class="flex items-center gap-1">
            <select
                :value="sort"
                class="border-border bg-panel rounded-md border px-2 py-1.5 text-[11px] text-white"
                aria-label="Sort"
                @change="
                    emit('update:sort', ($event.target as HTMLSelectElement).value as TrashSort)
                "
            >
                <option v-for="s in TRASH_SORTS" :key="s.value" :value="s.value">
                    {{ s.label }}
                </option>
            </select>
            <button
                class="border-border text-dim rounded-md border p-1.5 transition-colors
                    hover:text-white"
                :title="order === 'asc' ? 'Ascending' : 'Descending'"
                @click="emit('toggleOrder')"
            >
                <Icon
                    :name="
                        order === 'asc' ? 'heroicons:bars-arrow-up' : 'heroicons:bars-arrow-down'
                    "
                    size="14"
                />
            </button>
        </div>
        <button
            class="text-dim hover:text-lava p-1 transition-colors"
            title="Refresh"
            @click="emit('refresh')"
        >
            <Icon name="heroicons:arrow-path" size="16" />
        </button>
    </div>
</template>
