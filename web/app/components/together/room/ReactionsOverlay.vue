<script setup lang="ts">
import type { ReactionEvent } from '~/composables/useTogetherSocket';

// Floating reactions over the video for everyone (teleported into the video.js element so
// that they also show in fullscreen). Bounded: the oldest go when too many float at once.
const props = defineProps<{
    target: HTMLElement | null;
}>();

const MAX_FLOATING = 30;
const DURATION_MS = 3200;

interface Floating {
    id: number;
    emote: string;
    username: string;
    left: number;
    drift: number;
    scale: number;
}

const { socket } = useInjectedTogetherRoom();
const { byName } = useTogetherEmotes();

const floating = ref<Floating[]>([]);
let nextId = 0;
const timers = new Set<ReturnType<typeof setTimeout>>();

const add = (r: ReactionEvent) => {
    const item: Floating = {
        id: ++nextId,
        emote: r.emote,
        username: r.username,
        left: 8 + Math.random() * 84,
        drift: (Math.random() * 2 - 1) * 40,
        scale: 0.85 + Math.random() * 0.4,
    };
    const next = [...floating.value, item];
    floating.value = next.length > MAX_FLOATING ? next.slice(next.length - MAX_FLOATING) : next;
    const timer = setTimeout(() => {
        timers.delete(timer);
        floating.value = floating.value.filter((f) => f.id !== item.id);
    }, DURATION_MS);
    timers.add(timer);
};

const off = socket.onReaction(add);
onBeforeUnmount(() => {
    off();
    for (const t of timers) clearTimeout(t);
    timers.clear();
});

const emoteUrl = (value: string) =>
    isCustomEmoteReaction(value)
        ? (byName.value.get(value.slice(1, -1).toLowerCase())?.url ?? null)
        : null;
</script>

<template>
    <Teleport :to="props.target ?? 'body'" :disabled="!props.target">
        <div class="together-reactions" aria-hidden="true">
            <div
                v-for="f in floating"
                :key="f.id"
                class="together-reaction"
                :style="{
                    left: `${f.left}%`,
                    '--drift': `${f.drift}px`,
                    '--scale': f.scale,
                    animationDuration: `${DURATION_MS}ms`,
                }"
            >
                <img
                    v-if="emoteUrl(f.emote)"
                    :src="emoteUrl(f.emote) ?? undefined"
                    :alt="f.emote"
                    class="h-10 w-10 object-contain"
                />
                <span v-else class="text-3xl leading-none">{{ f.emote }}</span>
                <span class="together-reaction__name">{{ f.username }}</span>
            </div>
        </div>
    </Teleport>
</template>

<style scoped>
.together-reactions {
    position: absolute;
    inset: 0;
    pointer-events: none;
    overflow: hidden;
    z-index: 15;
}

.together-reaction {
    position: absolute;
    bottom: 12%;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    animation-name: together-float;
    animation-timing-function: cubic-bezier(0.2, 0.6, 0.4, 1);
    animation-fill-mode: forwards;
    filter: drop-shadow(0 2px 6px rgba(0, 0, 0, 0.6));
}

.together-reaction__name {
    font-size: 9px;
    color: rgba(255, 255, 255, 0.7);
    max-width: 80px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

@keyframes together-float {
    0% {
        transform: translate(-50%, 0) scale(calc(var(--scale) * 0.6));
        opacity: 0;
    }
    12% {
        transform: translate(-50%, -20px) scale(var(--scale));
        opacity: 1;
    }
    80% {
        opacity: 1;
    }
    100% {
        transform: translate(calc(-50% + var(--drift)), -260px) scale(var(--scale));
        opacity: 0;
    }
}

@media (prefers-reduced-motion: reduce) {
    .together-reaction {
        animation-name: together-fade;
    }
    @keyframes together-fade {
        0%,
        80% {
            opacity: 1;
            transform: translate(-50%, 0);
        }
        100% {
            opacity: 0;
            transform: translate(-50%, 0);
        }
    }
}
</style>
