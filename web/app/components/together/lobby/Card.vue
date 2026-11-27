<script setup lang="ts">
import type { LobbySummary } from '~/types/together';

// Lobby browser card: name, host, online members, now playing. Opening a lobby one is not a
// member of joins it; public lobbies can also be joined anonymously.
const props = defineProps<{
    lobby: LobbySummary;
    canClose?: boolean;
}>();

const emit = defineEmits<{
    close: [lobby: LobbySummary];
}>();

const link = computed(
    () => `/together/${props.lobby.id}${props.lobby.is_member ? '' : '?join=named'}`,
);
const canJoinAnonymously = computed(
    () => !props.lobby.is_member && props.lobby.visibility === 'public',
);
const joinAnonymously = () => navigateTo(`/together/${props.lobby.id}?join=anonymous`);

const thumb = computed(() =>
    props.lobby.now_playing ? `/thumbnails/${props.lobby.now_playing.scene_id}?size=sm` : '',
);
</script>

<template>
    <NuxtLink
        :to="link"
        class="group border-border bg-panel/60 hover:border-lava/30 relative flex gap-3
            overflow-hidden rounded-xl border p-3 transition-all"
    >
        <div class="bg-void relative h-16 w-28 shrink-0 overflow-hidden rounded-lg">
            <img
                v-if="thumb"
                :src="thumb"
                alt=""
                class="h-full w-full object-cover opacity-80 transition-opacity
                    group-hover:opacity-100"
                loading="lazy"
            />
            <div v-else class="text-dim flex h-full items-center justify-center">
                <Icon name="heroicons:film" size="18" />
            </div>
            <span
                v-if="lobby.online_count > 0"
                class="bg-void/80 absolute bottom-1 left-1 flex items-center gap-1 rounded px-1
                    py-px text-[9px] font-semibold text-white backdrop-blur-sm"
            >
                <span class="bg-emerald h-1.5 w-1.5 rounded-full" />
                LIVE
            </span>
        </div>
        <div class="min-w-0 flex-1">
            <div class="flex items-center gap-1.5">
                <Icon
                    :name="
                        lobby.visibility === 'public'
                            ? 'heroicons:globe-alt'
                            : 'heroicons:lock-closed'
                    "
                    size="11"
                    class="text-dim shrink-0"
                />
                <h3 class="truncate text-sm font-semibold text-white">{{ lobby.name }}</h3>
                <span
                    v-if="lobby.anonymous"
                    class="border-border text-dim flex shrink-0 items-center gap-0.5 rounded border
                        px-1 py-px text-[9px]"
                    title="You joined this lobby anonymously"
                >
                    <Icon name="heroicons:eye-slash" size="9" />
                    Anonymous
                </span>
            </div>
            <div class="text-dim mt-0.5 truncate text-[11px]">
                <template v-if="lobby.host">hosted by {{ lobby.host.username }}</template>
                <template v-else>no host</template>
            </div>
            <div class="text-dim mt-1.5 flex items-center gap-3 text-[10px]">
                <span class="flex items-center gap-1">
                    <Icon name="heroicons:users" size="11" class="text-lava" />
                    {{ lobby.online_count }} online
                </span>
                <span class="flex items-center gap-1">
                    <Icon name="heroicons:queue-list" size="11" class="text-lava" />
                    {{ lobby.queue_length }} queued
                </span>
            </div>
            <div v-if="lobby.now_playing?.title" class="mt-1 truncate text-[11px] text-white/80">
                <Icon name="heroicons:play" size="10" class="text-lava mr-0.5 align-[-1px]" />
                {{ lobby.now_playing.label || lobby.now_playing.title }}
            </div>
            <button
                v-if="canJoinAnonymously"
                class="text-dim hover:text-lava mt-1.5 flex items-center gap-1 text-[10px]
                    transition-colors"
                title="The other members see you as Anonymous #N, without your name or profile"
                @click.prevent.stop="joinAnonymously"
            >
                <Icon name="heroicons:eye-slash" size="11" />
                Join anonymously
            </button>
        </div>

        <button
            v-if="canClose"
            class="text-dim absolute top-2 right-2 flex h-6 w-6 items-center justify-center rounded
                opacity-0 transition-all group-hover:opacity-100 hover:text-red-400"
            title="Close the lobby"
            @click.prevent.stop="emit('close', lobby)"
        >
            <Icon name="heroicons:power" size="13" />
        </button>
    </NuxtLink>
</template>
