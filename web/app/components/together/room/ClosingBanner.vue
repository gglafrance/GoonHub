<script setup lang="ts">
// The server announced that the lobby closes for inactivity: a countdown, and a button that
// keeps it open (any activity does: playing, chatting, changing the queue).
const { socket } = useInjectedTogetherRoom();

const now = ref(Date.now());
let timer: ReturnType<typeof setInterval> | null = null;

const left = computed(() => {
    const at = socket.closingAt.value;
    if (at == null) return null;
    // now ticks every second; the offset turns it into the server clock
    return Math.max(0, Math.round((at - (now.value + socket.clockOffset.value)) / 1000));
});

watch(
    () => socket.closingAt.value,
    (at) => {
        if (timer) clearInterval(timer);
        timer = null;
        if (at == null) return;
        now.value = Date.now();
        timer = setInterval(() => (now.value = Date.now()), 1000);
    },
    { immediate: true },
);
onBeforeUnmount(() => {
    if (timer) clearInterval(timer);
});

const keepOpen = () => socket.send({ t: 'keep_open' });
</script>

<template>
    <div
        v-if="left !== null"
        class="flex items-center gap-2 rounded-lg border border-amber-400/30 bg-amber-400/5 px-3
            py-2 text-xs text-white"
        role="status"
    >
        <Icon name="heroicons:clock" size="14" class="shrink-0 text-amber-400" />
        <span class="min-w-0 flex-1">
            Nothing is happening: this lobby closes in
            <span class="font-mono font-semibold text-amber-400">{{ formatClock(left) }}</span>
            for inactivity.
        </span>
        <button
            class="shrink-0 rounded-md border border-amber-400/40 px-2.5 py-1 text-[11px]
                font-medium text-amber-400 transition-colors hover:bg-amber-400/10"
            @click="keepOpen"
        >
            Keep it open
        </button>
    </div>
</template>
