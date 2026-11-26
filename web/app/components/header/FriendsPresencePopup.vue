<script setup lang="ts">
// Popover of the friends presence: contacts grouped Watching / Online / Idle / Offline.
const props = defineProps<{ visible: boolean; anchorEl: HTMLElement | null }>();
const emit = defineEmits<{ close: [] }>();

const presenceStore = usePresenceStore();
const popupRef = ref<HTMLDivElement | null>(null);
const position = ref({ top: 0, left: 0 });
// clock of the playback progress (ticks while open)
const now = ref(Date.now());
let clock: ReturnType<typeof setInterval> | null = null;

const groups = computed(() => groupPresence(presenceStore.contacts));

function updatePosition() {
    if (!props.anchorEl) return;
    const rect = props.anchorEl.getBoundingClientRect();
    const width = 320;
    const padding = 8;
    let left = rect.left + rect.width / 2 - width / 2;
    left = Math.max(padding, Math.min(left, window.innerWidth - width - padding));
    position.value = { top: rect.bottom + 8, left };
}

function onClickOutside(e: MouseEvent) {
    if (
        popupRef.value &&
        !popupRef.value.contains(e.target as Node) &&
        props.anchorEl &&
        !props.anchorEl.contains(e.target as Node)
    ) {
        emit('close');
    }
}

function teardown() {
    window.removeEventListener('resize', updatePosition);
    document.removeEventListener('mousedown', onClickOutside);
    if (clock) clearInterval(clock);
    clock = null;
}

watch(
    () => props.visible,
    (visible) => {
        if (visible) {
            updatePosition();
            now.value = Date.now();
            clock = setInterval(() => (now.value = Date.now()), 5000);
            window.addEventListener('resize', updatePosition);
            document.addEventListener('mousedown', onClickOutside);
        } else {
            teardown();
        }
    },
);

onBeforeUnmount(teardown);
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            ref="popupRef"
            class="border-border bg-panel/95 fixed z-50 flex max-h-[70vh] w-80 flex-col rounded-lg
                border shadow-2xl backdrop-blur-md"
            :style="{ top: `${position.top}px`, left: `${position.left}px` }"
        >
            <div class="border-border flex items-center justify-between border-b px-4 py-3">
                <div class="flex items-center gap-2">
                    <Icon name="heroicons:signal" size="16" class="text-lava" />
                    <span class="text-xs font-semibold text-white">Friends online</span>
                </div>
                <NuxtLink
                    to="/friends"
                    class="text-dim hover:text-lava text-[10px] transition-colors"
                    @click="emit('close')"
                >
                    All friends
                </NuxtLink>
            </div>

            <NuxtLink
                v-if="presenceStore.invisible"
                to="/settings?tab=privacy"
                class="border-border text-dim hover:text-lava flex items-center gap-1.5 border-b
                    px-4 py-2 text-[10px] transition-colors"
                @click="emit('close')"
            >
                <Icon name="heroicons:eye-slash" size="12" />
                You are invisible: others see you offline
            </NuxtLink>

            <div class="min-h-0 flex-1 overflow-y-auto py-1">
                <div v-if="!presenceStore.loaded" class="flex justify-center py-6">
                    <LoadingSpinner />
                </div>
                <div
                    v-else-if="groups.length === 0"
                    class="text-dim px-4 py-6 text-center text-[11px]"
                >
                    Your friends and the people you follow show up here, when they let you see their
                    status.
                </div>
                <div v-for="g in groups" :key="g.key" class="py-1">
                    <div
                        class="px-4 pt-1 pb-0.5 text-[10px] font-medium tracking-wider text-white/40
                            uppercase"
                    >
                        {{ g.label }} <span class="text-white/25">{{ g.views.length }}</span>
                    </div>
                    <SocialPresenceEntry
                        v-for="v in g.views"
                        :key="v.user.id"
                        :view="v"
                        :now="now"
                        @navigate="emit('close')"
                    />
                </div>
            </div>
        </div>
    </Teleport>
</template>
