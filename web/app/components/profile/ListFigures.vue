<script setup lang="ts">
import type { ListFigureKind, ProfileSceneFigures } from '~/types/profile';

// The owner's figures under a card of a custom list (block and Browse page): stars for the
// rating, "12 O · last 2 days ago", "Liked 2 days ago", "34 views · last 3 days ago". One line
// per figure (two at most), truncated to the card width.
const props = defineProps<{
    kinds?: ListFigureKind[];
    figures?: ProfileSceneFigures;
}>();

const lines = computed(() => listFigureLines(props.kinds, props.figures));
</script>

<template>
    <div v-if="lines.length" class="mt-1 min-w-0 space-y-0.5">
        <div
            v-for="line in lines"
            :key="line.kind"
            class="text-dim flex min-w-0 items-center gap-1 text-[10px] leading-tight"
        >
            <span
                v-if="line.stars"
                class="text-lava shrink-0 text-xs leading-none"
                :title="line.title"
                >{{ line.stars }}</span
            >
            <Icon
                v-if="line.kind === 'jizz'"
                name="fluent-emoji-high-contrast:sweat-droplets"
                size="11"
                class="text-lava shrink-0"
            />
            <Icon
                v-else-if="line.kind === 'liked'"
                name="heroicons:heart-solid"
                size="10"
                class="text-lava shrink-0"
            />
            <span v-if="line.value" class="shrink-0 font-mono text-white">{{ line.value }}</span>
            <span v-if="line.at" class="min-w-0 truncate">
                <template v-if="line.value">· </template>{{ line.prefix }}
                <NuxtTime :datetime="line.at" relative />
            </span>
        </div>
    </div>
</template>
