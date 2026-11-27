<script setup lang="ts">
// Logout (required header item): icon only on mobile.
const authStore = useAuthStore();
</script>

<template>
    <button
        class="border-border text-dim hover:border-lava/30 hover:text-lava flex h-7 w-7 items-center
            justify-center rounded-md border transition-all sm:h-auto sm:w-auto sm:px-2.5 sm:py-1"
        @click="authStore.logout()"
    >
        <Icon name="heroicons:arrow-right-on-rectangle" size="16" class="sm:hidden" />
        <span class="ml-1.5 hidden text-[11px] font-medium sm:inline">Logout</span>
    </button>
</template>
