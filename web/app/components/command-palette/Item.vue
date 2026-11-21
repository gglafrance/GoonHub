<script setup lang="ts">
import type { PaletteItem } from '~/types/command-palette';

const props = defineProps<{
    item: PaletteItem;
    active: boolean;
}>();

defineEmits<{
    select: [event: MouseEvent];
    hover: [];
}>();

const imageFailed = ref(false);
watch(
    () => props.item.image,
    () => {
        imageFailed.value = false;
    },
);

const imageClass = computed(() => {
    switch (props.item.imageShape) {
        case 'portrait':
            return 'h-9 w-6';
        case 'square':
            return 'h-8 w-8 bg-white/5 object-contain p-0.5';
        default:
            return 'h-9 w-16';
    }
});
</script>

<template>
    <button
        type="button"
        role="option"
        :aria-selected="active"
        class="group relative flex w-full items-center gap-3 rounded-lg px-2.5 py-1.5 text-left
            transition-colors"
        :class="active ? 'bg-lava/10' : 'hover:bg-white/3'"
        @mousedown.prevent
        @mousemove="$emit('hover')"
        @click="$emit('select', $event)"
    >
        <span
            v-if="active"
            class="bg-lava absolute top-1.5 bottom-1.5 left-0 w-0.5 rounded-full"
            aria-hidden="true"
        />

        <img
            v-if="item.image && !imageFailed"
            :src="item.image"
            alt=""
            loading="lazy"
            class="border-border shrink-0 rounded border object-cover"
            :class="imageClass"
            @error="imageFailed = true"
        />
        <span
            v-else
            class="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border
                transition-colors"
            :class="
                active
                    ? 'border-lava/30 bg-lava/10 text-lava'
                    : 'border-border bg-panel text-dim group-hover:text-white'
            "
        >
            <Icon :name="item.icon" size="15" />
        </span>

        <span class="min-w-0 flex-1">
            <span
                class="block truncate text-[13px] font-medium"
                :class="active ? 'text-white' : 'text-white/85'"
            >
                {{ item.label }}
            </span>
            <span v-if="item.sublabel" class="text-dim block truncate text-[11px]">
                {{ item.sublabel }}
            </span>
        </span>

        <span
            v-if="item.hint"
            class="border-border text-dim shrink-0 rounded border px-1.5 py-0.5 font-mono text-[9px]
                tracking-wider uppercase"
        >
            {{ item.hint }}
        </span>
        <Icon
            v-if="active"
            name="heroicons:arrow-turn-down-left"
            size="14"
            class="text-lava shrink-0"
        />
    </button>
</template>
