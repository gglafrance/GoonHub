<script setup lang="ts">
const searchStore = useSearchStore();

const collapsed = ref(true);

const badge = computed(() => {
    if (!searchStore.minDate && !searchStore.maxDate) return undefined;
    return `${searchStore.minDate || '*'} - ${searchStore.maxDate || '*'}`;
});
</script>

<template>
    <SearchFiltersFilterSection
        title="Date"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <div class="space-y-1.5">
            <input
                v-model="searchStore.minDate"
                type="date"
                class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5 text-xs
                    focus:border-white/20 focus:outline-none"
            />
            <input
                v-model="searchStore.maxDate"
                type="date"
                class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5 text-xs
                    focus:border-white/20 focus:outline-none"
            />
        </div>
    </SearchFiltersFilterSection>
</template>
