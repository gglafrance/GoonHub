<script setup lang="ts">
const authStore = useAuthStore();
const { connect, disconnect } = useSSE();

watch(
    () => authStore.isAuthenticated,
    (isAuth) => {
        if (isAuth) {
            connect();
        } else {
            disconnect();
        }
    },
    { immediate: true },
);

onBeforeUnmount(() => {
    disconnect();
});
</script>

<template>
    <div class="cosmic-bg min-h-screen">
        <AppHeader />
        <NuxtPage />
        <UploadIndicator />
    </div>
</template>
