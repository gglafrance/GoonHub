<script setup lang="ts">
import type { LobbySummary } from '~/types/together';

// Join screen of a lobby the user is not a member of (opened from a link): join under one's
// name, or anonymously (public lobbies only: the others see "Anonymous #N").
const props = defineProps<{
    lobby: LobbySummary;
    busy: boolean;
    error: string;
}>();

const emit = defineEmits<{
    join: [anonymous: boolean];
}>();

const canBeAnonymous = computed(() => props.lobby.visibility === 'public');
</script>

<template>
    <div class="flex h-[60vh] flex-col items-center justify-center text-center">
        <div
            class="border-lava/20 bg-lava/5 flex h-12 w-12 items-center justify-center rounded-xl
                border"
        >
            <Icon name="heroicons:user-group" size="24" class="text-lava" />
        </div>
        <h2 class="mt-4 text-lg font-semibold text-white">{{ lobby.name }}</h2>
        <p class="text-dim mt-1 text-xs">
            <template v-if="lobby.host">Hosted by {{ lobby.host.username }} · </template>
            {{ lobby.online_count }} online · {{ lobby.queue_length }} queued
        </p>
        <div class="mt-6 flex flex-wrap items-center justify-center gap-2">
            <button
                class="bg-lava hover:bg-lava/90 flex items-center gap-1.5 rounded-lg px-4 py-2
                    text-xs font-medium text-white transition-colors disabled:opacity-50"
                :disabled="busy"
                @click="emit('join', false)"
            >
                <Icon name="heroicons:arrow-right-end-on-rectangle" size="14" />
                Join
            </button>
            <button
                v-if="canBeAnonymous"
                class="border-border bg-panel hover:border-lava/30 flex items-center gap-1.5
                    rounded-lg border px-4 py-2 text-xs font-medium text-white transition-all
                    disabled:opacity-50"
                :disabled="busy"
                title="The other members see you as Anonymous #N, without your name or profile"
                @click="emit('join', true)"
            >
                <Icon name="heroicons:eye-slash" size="14" />
                Join anonymously
            </button>
        </div>
        <p v-if="canBeAnonymous" class="text-dim mt-3 max-w-sm text-[11px]">
            Anonymously, the other members see you as "Anonymous #N", without your name or profile
            (moderators can still see who you are).
        </p>
        <p v-if="error" class="mt-3 max-w-sm text-xs text-red-400">{{ error }}</p>
        <NuxtLink
            to="/together"
            class="text-dim mt-6 text-[11px] transition-colors hover:text-white"
        >
            Back to the lobbies
        </NuxtLink>
    </div>
</template>
