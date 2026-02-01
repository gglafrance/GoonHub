<script setup lang="ts">
const jobStatusStore = useJobStatusStore();
const showPopup = ref(false);
const buttonRef = ref<HTMLButtonElement | null>(null);
const route = useRoute();

// Auto-hide popup on route change
watch(
    () => route.path,
    () => {
        showPopup.value = false;
    },
);

const badgeText = computed(() => {
    const r = jobStatusStore.totalRunning;
    const w = jobStatusStore.totalWaiting; // queued + pending
    if (r === 0 && w === 0) return '';
    return `${r}/${w}`;
});

function togglePopup() {
    showPopup.value = !showPopup.value;
}
</script>

<template>
    <div class="relative">
        <button
            ref="buttonRef"
            class="border-border text-dim hover:border-lava/30 hover:text-lava flex h-7 items-center
                gap-1.5 rounded-md border px-2 transition-all"
            :class="{
                'border-lava/30 text-lava': jobStatusStore.isActive,
                'opacity-50': !jobStatusStore.isConnected,
            }"
            title="Job Status"
            @click="togglePopup"
        >
            <Icon
                name="heroicons:bolt"
                size="14"
                :class="{ 'animate-pulse': jobStatusStore.isActive }"
            />
            <span v-if="badgeText" class="font-mono text-[10px]">{{ badgeText }}</span>
            <span v-else class="text-dim font-mono text-[10px]">0/0</span>
        </button>

        <HeaderJobStatusPopup
            :visible="showPopup"
            :anchor-el="buttonRef"
            @close="showPopup = false"
        />
    </div>
</template>
