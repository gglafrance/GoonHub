<script setup lang="ts">
const authStore = useAuthStore();
const { connect, disconnect } = useSSE();
const { startAuthValidation, stopAuthValidation } = useAuthValidation();

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

onMounted(() => {
    startAuthValidation();
});

onBeforeUnmount(() => {
    disconnect();
    stopAuthValidation();
});
</script>

<template>
    <div class="cosmic-bg min-h-screen overflow-x-hidden">
        <AppHeader />
        <NuxtPage />
        <UploadIndicator />
    </div>
</template>
