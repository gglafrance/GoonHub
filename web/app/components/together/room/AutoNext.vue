<script setup lang="ts">
import type { AutoNextMode } from '~/types/together';

// What happens at the end of an item (lobby setting, synced to everyone): anyone with control
// changes it, the others see it.
const { socket, canControl } = useInjectedTogetherRoom();

const open = ref(false);
const root = ref<HTMLElement | null>(null);

const current = computed(() => autoNextMode(socket.lobby.value?.auto_next));

const choose = (mode: AutoNextMode) => {
    open.value = false;
    if (mode !== current.value.value) socket.send({ t: 'auto_next', mode });
};

const toggle = () => {
    if (canControl.value) open.value = !open.value;
};

const onDocumentClick = (e: MouseEvent) => {
    if (open.value && root.value && !root.value.contains(e.target as Node)) open.value = false;
};
onMounted(() => document.addEventListener('click', onDocumentClick));
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick));
</script>

<template>
    <div ref="root" class="relative">
        <button
            class="border-border bg-panel flex h-7 items-center gap-1.5 rounded-md border px-2.5
                text-[11px] font-medium transition-all"
            :class="
                canControl
                    ? 'hover:border-lava/30 hover:text-lava text-white'
                    : 'text-dim cursor-default'
            "
            :title="
                canControl
                    ? `At the end of an item: ${current.hint}`
                    : `At the end of an item: ${current.hint} (changed by members with control)`
            "
            :aria-expanded="open"
            @click.stop="toggle"
        >
            <Icon :name="current.icon" size="13" />
            <span>{{ current.label }}</span>
            <Icon v-if="canControl" name="heroicons:chevron-down" size="11" class="text-dim" />
        </button>
        <div
            v-if="open"
            class="border-border bg-panel/95 absolute right-0 bottom-full z-30 mb-1 w-60
                overflow-hidden rounded-lg border shadow-2xl backdrop-blur-md"
        >
            <div class="text-dim px-3 pt-2 pb-1 text-[10px] font-semibold tracking-wider uppercase">
                At the end of an item
            </div>
            <button
                v-for="m in AUTO_NEXT_MODES"
                :key="m.value"
                class="flex w-full items-start gap-2 px-3 py-1.5 text-left transition-colors
                    hover:bg-white/5"
                @click="choose(m.value)"
            >
                <Icon
                    :name="m.icon"
                    size="13"
                    class="mt-0.5 shrink-0"
                    :class="m.value === current.value ? 'text-lava' : 'text-dim'"
                />
                <span class="min-w-0">
                    <span
                        class="block text-xs"
                        :class="m.value === current.value ? 'text-lava' : 'text-white'"
                    >
                        {{ m.label }}
                    </span>
                    <span class="text-dim block text-[10px]">{{ m.hint }}</span>
                </span>
            </button>
        </div>
    </div>
</template>
