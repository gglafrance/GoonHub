<script setup lang="ts">
const searchStore = useSearchStore();

const collapsed = ref(true);

const badge = computed(() => {
    if (searchStore.minRating <= 0 && searchStore.maxRating <= 0) return undefined;
    if (searchStore.minRating && searchStore.maxRating) {
        return `${searchStore.minRating}-${searchStore.maxRating}`;
    }
    if (searchStore.minRating) {
        return `${searchStore.minRating}+`;
    }
    return `<${searchStore.maxRating}`;
});
</script>

<template>
    <SearchFiltersFilterSection
        title="Rating"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <div class="flex items-center gap-1.5">
            <input
                v-model.number="searchStore.minRating"
                type="number"
                min="0"
                max="5"
                step="0.5"
                placeholder="Min"
                class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5 text-xs
                    focus:border-white/20 focus:outline-none"
            />
            <span class="text-dim text-[10px]">-</span>
            <input
                v-model.number="searchStore.maxRating"
                type="number"
                min="0"
                max="5"
                step="0.5"
                placeholder="Max"
                class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5 text-xs
                    focus:border-white/20 focus:outline-none"
            />
        </div>
        <p class="text-dim mt-1.5 text-[10px]">Scale: 0.5 - 5</p>
    </SearchFiltersFilterSection>
</template>
