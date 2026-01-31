<script setup lang="ts">
const searchStore = useSearchStore();

const collapsed = ref(true);

const badge = computed(() => {
    if (searchStore.minJizzCount <= 0 && searchStore.maxJizzCount <= 0) return undefined;
    if (searchStore.minJizzCount && searchStore.maxJizzCount) {
        return `${searchStore.minJizzCount}-${searchStore.maxJizzCount}`;
    }
    if (searchStore.minJizzCount) {
        return `${searchStore.minJizzCount}+`;
    }
    return `<${searchStore.maxJizzCount}`;
});
</script>

<template>
    <SearchFiltersFilterSection
        title="Jizz Count"
        icon="fluent-emoji-high-contrast:sweat-droplets"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <div class="flex items-center gap-1.5">
            <input
                v-model.number="searchStore.minJizzCount"
                type="number"
                min="0"
                placeholder="Min"
                class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5
                    text-xs focus:border-white/20 focus:outline-none"
            />
            <span class="text-dim text-[10px]">-</span>
            <input
                v-model.number="searchStore.maxJizzCount"
                type="number"
                min="0"
                placeholder="Max"
                class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5
                    text-xs focus:border-white/20 focus:outline-none"
            />
        </div>
    </SearchFiltersFilterSection>
</template>
