<script setup lang="ts">
const searchStore = useSearchStore();

const collapsed = ref(true);

const badge = computed(() => (searchStore.liked ? 'Yes' : undefined));
</script>

<template>
    <SearchFiltersFilterSection
        title="Liked"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <label class="flex cursor-pointer items-center gap-2">
            <input
                v-model="searchStore.liked"
                type="checkbox"
                class="accent-lava h-3.5 w-3.5 rounded"
            />
            <span class="text-dim text-xs">Only show liked videos</span>
        </label>
    </SearchFiltersFilterSection>
</template>
