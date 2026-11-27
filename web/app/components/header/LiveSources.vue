<script setup lang="ts">
// Live state behind the header items, started once whatever the user's layout: an item can be
// hidden, put in a folder or shown twice (the settings preview) and still have its badge.
// Social summary (friend requests, the header avatar), messages (unread count, toasts),
// presence (also reports this tab's activity), public lobby count (Together) and the header
// layout itself. Refreshes them all when the live stream comes back.
const perms = useMyPermissions();
const jobStatusStore = useJobStatusStore();
const socialStore = useSocialStore();
const messagesStore = useMessagesStore();
const presenceStore = usePresenceStore();
const togetherLobbies = useTogetherLobbiesStore();
const navLayoutStore = useNavLayoutStore();

const canTogether = computed(() => perms.has('goon2gether:use'));

onMounted(() => {
    socialStore.start();
    messagesStore.start();
    presenceStore.start();
    navLayoutStore.start();
});

watch(
    canTogether,
    (can) => {
        if (can) togetherLobbies.start();
        else togetherLobbies.stop();
    },
    { immediate: true },
);

onBeforeUnmount(() => {
    socialStore.stop();
    messagesStore.stop();
    presenceStore.stop();
    togetherLobbies.stop();
    navLayoutStore.stop();
});

// Events may have been missed while the stream was down
watch(
    () => jobStatusStore.lastReconnectedAt,
    () => {
        socialStore.refresh();
        messagesStore.reconnected();
        presenceStore.refresh();
        navLayoutStore.refresh();
        if (canTogether.value) togetherLobbies.refresh();
    },
);
</script>

<template>
    <MessagesToasts />
</template>
