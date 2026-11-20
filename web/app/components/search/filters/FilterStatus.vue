<script setup lang="ts">
import type { SearchStatusKey } from '~/stores/search';

// Library status filters: each criterion is "any", "yes" or "no".
const searchStore = useSearchStore();

const collapsed = ref(true);

const rows: { key: SearchStatusKey; label: string; yes: string; no: string }[] = [
    { key: 'watched', label: 'Watched', yes: 'Watched', no: 'Never watched' },
    { key: 'rated', label: 'Rated', yes: 'Rated', no: 'Not rated' },
    { key: 'has_markers', label: 'Markers', yes: 'With markers', no: 'Without markers' },
    { key: 'has_actors', label: 'Actors', yes: 'With actors', no: 'Without actors' },
    { key: 'has_tags', label: 'Tags', yes: 'With tags', no: 'Without tags' },
    { key: 'has_studio', label: 'Studio', yes: 'With studio', no: 'Without studio' },
];

const options: { value: boolean | null; label: string }[] = [
    { value: null, label: 'Any' },
    { value: true, label: 'Yes' },
    { value: false, label: 'No' },
];

const badge = computed(() => {
    const count = rows.filter((r) => searchStore.status[r.key] !== null).length;
    return count > 0 ? count : undefined;
});
</script>

<template>
    <SearchFiltersFilterSection
        title="Library Status"
        icon="heroicons:check-badge"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <div class="space-y-1.5">
            <div v-for="row in rows" :key="row.key" class="flex items-center justify-between gap-2">
                <span class="text-dim text-[11px]">{{ row.label }}</span>
                <div class="border-border flex rounded-md border p-0.5">
                    <button
                        v-for="opt in options"
                        :key="String(opt.value)"
                        class="rounded px-1.5 py-0.5 text-[10px] font-medium transition-colors"
                        :class="
                            searchStore.status[row.key] === opt.value
                                ? 'bg-lava/15 text-lava'
                                : 'text-dim hover:text-white'
                        "
                        :title="opt.value === true ? row.yes : opt.value === false ? row.no : ''"
                        @click="searchStore.status[row.key] = opt.value"
                    >
                        {{ opt.label }}
                    </button>
                </div>
            </div>
        </div>
    </SearchFiltersFilterSection>
</template>
