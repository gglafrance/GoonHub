<script setup lang="ts">
import type { LobbyVisibility } from '~/types/together';

// Lobby setting fields shared by the create modal and the room settings modal.
const name = defineModel<string>('name', { required: true });
const visibility = defineModel<LobbyVisibility>('visibility', { required: true });
const controlOpen = defineModel<boolean>('controlOpen', { required: true });
const queueOpen = defineModel<boolean>('queueOpen', { required: true });
const waitForAll = defineModel<boolean>('waitForAll', { required: true });
const maxMembers = defineModel<number>('maxMembers', { required: true });

const visibilityOptions = [
    {
        key: 'private' as const,
        label: 'Private',
        icon: 'heroicons:lock-closed',
        hint: 'Invite only',
    },
    { key: 'public' as const, label: 'Public', icon: 'heroicons:globe-alt', hint: 'Listed' },
];

const toggles = computed(() => [
    {
        model: controlOpen,
        label: 'Everyone controls playback',
        hint: 'Every member can play, pause, seek and skip',
    },
    {
        model: queueOpen,
        label: 'Everyone adds to the queue',
        hint: 'Otherwise members suggest scenes for the host to accept',
    },
    {
        model: waitForAll,
        label: 'Wait for everyone',
        hint: 'Pause while a member is buffering (at most a few seconds)',
    },
]);
</script>

<template>
    <div class="space-y-4">
        <div>
            <label class="text-dim mb-1 block text-[11px] font-medium">Name</label>
            <input
                v-model="name"
                type="text"
                maxlength="100"
                placeholder="Friday night"
                class="border-border bg-surface focus:border-lava/40 w-full rounded-lg border px-3
                    py-2 text-xs text-white outline-none"
            />
        </div>
        <div>
            <label class="text-dim mb-1 block text-[11px] font-medium">Visibility</label>
            <div class="grid grid-cols-2 gap-2">
                <button
                    v-for="o in visibilityOptions"
                    :key="o.key"
                    type="button"
                    class="flex items-center gap-2 rounded-lg border px-3 py-2 text-left
                        transition-all"
                    :class="
                        visibility === o.key
                            ? 'border-lava/40 bg-lava/10 text-white'
                            : 'border-border text-dim hover:text-white'
                    "
                    @click="visibility = o.key"
                >
                    <Icon :name="o.icon" size="14" />
                    <span class="text-xs font-medium">{{ o.label }}</span>
                    <span class="text-dim ml-auto text-[10px]">{{ o.hint }}</span>
                </button>
            </div>
        </div>
        <div class="space-y-3">
            <div v-for="t in toggles" :key="t.label" class="flex items-center gap-3">
                <div class="min-w-0 flex-1">
                    <div class="text-xs text-white">{{ t.label }}</div>
                    <div class="text-dim text-[10px]">{{ t.hint }}</div>
                </div>
                <UiToggle v-model="t.model.value" />
            </div>
        </div>
        <div class="flex items-center gap-3">
            <div class="min-w-0 flex-1">
                <div class="text-xs text-white">Max members</div>
                <div class="text-dim text-[10px]">0 uses the server limit</div>
            </div>
            <input
                v-model.number="maxMembers"
                type="number"
                min="0"
                max="100"
                class="border-border bg-surface focus:border-lava/40 w-20 rounded-lg border px-2
                    py-1 text-right text-xs text-white outline-none"
            />
        </div>
    </div>
</template>
