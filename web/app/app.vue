<script setup lang="ts">
const authStore = useAuthStore();
const perms = useMyPermissions();
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

const { init: initSafeMode } = useSafeMode();
const { init: initHoverAudio } = useHoverAudio();

onMounted(() => {
    startAuthValidation();
    initSafeMode();
    initHoverAudio();
});

onBeforeUnmount(() => {
    disconnect();
    stopAuthValidation();
});
</script>

<template>
    <!-- overflow-x clip on phones (hidden would make this div the scroll container of the sticky
         header and bars, which then never stick); the safe-area insets matter in landscape -->
    <div
        class="cosmic-bg min-h-screen overflow-x-clip pr-[env(safe-area-inset-right)]
            pl-[env(safe-area-inset-left)] md:overflow-x-hidden"
    >
        <AppHeader />
        <NuxtPage />
        <UploadIndicator v-if="perms.has('scenes:upload')" />
    </div>
</template>
