<script setup lang="ts">
const props = defineProps<{
    actors: string[];
    badge?: boolean;
}>();

const showPopover = ref(false);
const trigger = ref<HTMLElement | null>(null);
const popoverStyle = ref<Record<string, string>>({});
let hideTimeout: ReturnType<typeof setTimeout> | null = null;

function updatePosition() {
    if (!trigger.value) return;
    const rect = trigger.value.getBoundingClientRect();
    popoverStyle.value = {
        top: `${rect.bottom + 4}px`,
        left: `${rect.left}px`,
    };
}

function onTriggerEnter() {
    if (hideTimeout) { clearTimeout(hideTimeout); hideTimeout = null; }
    updatePosition();
    showPopover.value = true;
}

function onTriggerLeave() {
    hideTimeout = setTimeout(() => { showPopover.value = false; }, 100);
}

function onPopoverEnter() {
    if (hideTimeout) { clearTimeout(hideTimeout); hideTimeout = null; }
}

function onPopoverLeave() {
    showPopover.value = false;
}
</script>

<template>
    <div
        ref="trigger"
        class="relative inline-flex items-center"
        @mouseenter="onTriggerEnter"
        @mouseleave="onTriggerLeave"
    >
        <button
            class="flex items-center gap-0.5 transition-colors"
            :class="badge
                ? 'bg-void/90 rounded px-1.5 py-0.5 font-mono text-[10px] font-medium text-white backdrop-blur-sm hover:bg-void'
                : 'text-dim hover:text-lava text-[10px]'"
            @click.stop.prevent
        >
            <Icon name="heroicons:user" :size="badge ? '14' : '11'" />
            <span>{{ props.actors.length }}</span>
        </button>

        <Teleport to="body">
            <Transition
                enter-active-class="transition-opacity duration-100"
                enter-from-class="opacity-0"
                leave-active-class="transition-opacity duration-75"
                leave-to-class="opacity-0"
            >
                <div
                    v-if="showPopover"
                    class="bg-surface border-border fixed z-50 max-h-40 min-w-28 overflow-y-auto rounded-lg border p-1.5 shadow-lg backdrop-blur-md"
                    :style="popoverStyle"
                    @mouseenter="onPopoverEnter"
                    @mouseleave="onPopoverLeave"
                >
                    <NuxtLink
                        v-for="actor in props.actors"
                        :key="actor"
                        :to="`/actors/${encodeURIComponent(actor)}`"
                        class="text-dim hover:text-lava hover:bg-white/5 block truncate rounded px-1.5 py-0.5 text-[10px] transition-colors"
                        @click.stop
                    >
                        {{ actor }}
                    </NuxtLink>
                </div>
            </Transition>
        </Teleport>
    </div>
</template>
