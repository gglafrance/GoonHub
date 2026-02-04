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

const tooltipText = computed(() => {
    const parts: string[] = [];
    parts.push(`${jobStatusStore.totalRunning} running`);
    parts.push(`${jobStatusStore.totalWaiting} waiting`);
    if (jobStatusStore.totalFailed > 0) {
        parts.push(`${jobStatusStore.totalFailed} failed (1h)`);
    }
    return parts.join(', ');
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
                'border-red-500/40 text-red-400':
                    jobStatusStore.hasFailed && !jobStatusStore.isActive,
                'opacity-50': !jobStatusStore.isConnected,
            }"
            :title="tooltipText"
            @click="togglePopup"
        >
            <Icon
                name="heroicons:bolt"
                size="14"
                :class="{ 'animate-pulse': jobStatusStore.isActive }"
            />
            <span
                v-if="jobStatusStore.hasFailed && !badgeText"
                class="font-mono text-[10px] text-red-400"
            >
                {{ jobStatusStore.totalFailed }}!
            </span>
            <span v-else-if="badgeText" class="font-mono text-[10px]">{{ badgeText }}</span>
            <span v-else class="text-dim font-mono text-[10px]">0/0</span>
        </button>

        <HeaderJobStatusPopup
            :visible="showPopup"
            :anchor-el="buttonRef"
            @close="showPopup = false"
        />
    </div>
</template>
