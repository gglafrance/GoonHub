<script setup lang="ts">
// Header entry of the social features, with the pending incoming friend requests (live).
const socialStore = useSocialStore();
const jobStatusStore = useJobStatusStore();

const badge = computed(() => badgeCount(socialStore.incomingRequests));
const title = computed(() =>
    socialStore.incomingRequests > 0
        ? `Friends (${socialStore.incomingRequests} pending request${socialStore.incomingRequests > 1 ? 's' : ''})`
        : 'Friends',
);

onMounted(() => socialStore.start());
onBeforeUnmount(() => socialStore.stop());

// Events may have been missed while the stream was down
watch(
    () => jobStatusStore.lastReconnectedAt,
    () => socialStore.refresh(),
);
</script>

<template>
    <NuxtLink
        to="/friends"
        class="border-border text-dim hover:border-lava/30 hover:text-lava relative flex h-7 w-7
            items-center justify-center rounded-md border transition-all"
        active-class="!border-lava/30 !text-lava"
        :title="title"
        :aria-label="title"
    >
        <Icon name="heroicons:users" size="16" />
        <span
            v-if="badge"
            class="bg-lava absolute -top-1.5 -right-1.5 flex h-4 min-w-4 items-center justify-center
                rounded-full px-1 font-mono text-[9px] leading-none font-semibold text-white
                shadow-[0_0_8px_rgba(255,77,77,0.6)]"
        >
            {{ badge }}
        </span>
    </NuxtLink>
</template>
