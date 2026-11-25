<script setup lang="ts">
// User directory: every other user of the instance, filtered by username.
const api = useApiSocial();

const query = ref('');
const debounced = ref('');
let timer: ReturnType<typeof setTimeout> | null = null;

watch(query, (q) => {
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
        debounced.value = q.trim();
    }, 250);
});
onBeforeUnmount(() => {
    if (timer) clearTimeout(timer);
});

const loader = (page: number) => api.searchUsers(debounced.value, page);
</script>

<template>
    <div class="space-y-3">
        <div class="relative">
            <Icon
                name="heroicons:magnifying-glass"
                size="14"
                class="text-dim pointer-events-none absolute top-1/2 left-3 -translate-y-1/2"
            />
            <input
                v-model="query"
                type="search"
                maxlength="100"
                placeholder="Search users by name"
                class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                    focus:ring-lava/20 w-full rounded-lg border py-2 pr-3 pl-9 text-sm text-white
                    transition-all focus:ring-1 focus:outline-none"
            />
        </div>
        <SocialUserList
            :loader="loader"
            :reload-key="debounced"
            :empty-text="debounced ? `No user matches “${debounced}”.` : 'No other user yet.'"
        />
    </div>
</template>
