<script setup lang="ts">
// Header entry of the presence: how many friends and followed users are online, and a popover
// listing them (watching, online, away, offline). The live presence of the tab is started by
// HeaderLiveSources.
const presenceStore = usePresenceStore();
const route = useRoute();
const showPopup = ref(false);
const buttonRef = ref<HTMLButtonElement | null>(null);

// online, watching and away contacts; the dot is green when one of them is active, amber
// when they are all away
const count = computed(() => presenceStore.presentCount);
const active = computed(() => presenceStore.activeCount);
const title = computed(() => {
    if (count.value === 0) return 'Nobody online among your friends';
    const away = count.value - active.value;
    return away > 0 ? `${active.value} online, ${away} away` : `${count.value} online`;
});

watch(
    () => route.fullPath,
    () => {
        showPopup.value = false;
    },
);
</script>

<template>
    <div class="relative">
        <button
            ref="buttonRef"
            class="border-border text-dim hover:border-lava/30 hover:text-lava flex h-7 items-center
                gap-1.5 rounded-md border px-2 transition-all"
            :class="{ '!border-lava/30 !text-lava': showPopup }"
            :title="title"
            :aria-label="title"
            :aria-expanded="showPopup"
            @click="showPopup = !showPopup"
        >
            <Icon name="heroicons:signal" size="14" />
            <span
                class="h-1.5 w-1.5 rounded-full"
                :class="
                    active > 0
                        ? 'bg-emerald shadow-[0_0_6px_rgba(16,185,129,0.6)]'
                        : count > 0
                          ? 'bg-amber-400'
                          : 'bg-white/20'
                "
            />
            <span class="font-mono text-[10px]">{{ count }}</span>
        </button>
        <HeaderFriendsPresencePopup
            :visible="showPopup"
            :anchor-el="buttonRef"
            @close="showPopup = false"
        />
    </div>
</template>
