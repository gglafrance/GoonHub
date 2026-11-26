<script setup lang="ts">
// Suggestions of members without queue rights: controllers accept or reject them, authors
// can withdraw their own.
const room = useInjectedTogetherRoom();
const { socket, canControl, myId } = room;

const accept = (id: number) => socket.send({ t: 'suggestion_accept', item_id: id });
const reject = (id: number) => socket.send({ t: 'suggestion_reject', item_id: id });
</script>

<template>
    <div v-if="socket.suggestions.value.length > 0" class="space-y-1">
        <div
            class="text-dim flex items-center gap-1.5 px-1 text-[10px] font-semibold tracking-wider
                uppercase"
        >
            <Icon name="heroicons:light-bulb" size="11" class="text-amber-400" />
            Suggestions
            <span class="font-mono">{{ socket.suggestions.value.length }}</span>
        </div>
        <div
            v-for="s in socket.suggestions.value"
            :key="s.id"
            class="flex items-center gap-2 rounded-lg border border-amber-400/10 bg-amber-400/[0.03]
                px-2 py-1.5"
        >
            <img
                :src="`/thumbnails/${s.scene_id}?size=sm`"
                alt=""
                class="bg-void h-8 w-14 shrink-0 rounded object-cover"
                loading="lazy"
            />
            <div class="min-w-0 flex-1">
                <div class="truncate text-xs text-white" :title="s.title">{{ s.title }}</div>
                <div class="text-dim truncate text-[10px]">
                    <span v-if="s.label" class="text-lava/80">{{ s.label }} </span>
                    <span v-if="s.added_by">by {{ s.added_by.username }}</span>
                </div>
            </div>
            <button
                v-if="canControl"
                class="text-emerald flex h-6 w-6 items-center justify-center rounded
                    hover:bg-white/5"
                title="Accept"
                @click="accept(s.id)"
            >
                <Icon name="heroicons:check" size="14" />
            </button>
            <button
                v-if="canControl || s.added_by?.id === myId"
                class="text-dim flex h-6 w-6 items-center justify-center rounded hover:bg-white/5
                    hover:text-red-400"
                :title="canControl ? 'Reject' : 'Withdraw'"
                @click="reject(s.id)"
            >
                <Icon name="heroicons:x-mark" size="14" />
            </button>
        </div>
    </div>
</template>
