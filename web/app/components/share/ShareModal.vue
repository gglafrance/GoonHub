<script setup lang="ts">
import type { ShareTarget } from '~/types/messages';
import type { ShareSubject } from '~/types/share';

// The one Share dialog of the app (docs/MESSAGES.md, "Share dialog"): "Send to friends" (a
// message with a share card, messages:use) and "Link" (a share link, scenes:share, for a scene
// or a marker). A scene can start at a time ("Start at", prefilled with the player position);
// a marker shares its range.
const props = defineProps<{ visible: boolean; subject: ShareSubject }>();
const emit = defineEmits<{ close: [] }>();

const perms = useMyPermissions();
const canSend = computed(() => perms.has('messages:use'));
const canLink = computed(
    () =>
        perms.has('scenes:share') &&
        (props.subject.kind === 'scene' || props.subject.kind === 'marker'),
);
const tab = ref<'send' | 'link'>('send');
const tabs = [
    { id: 'send', label: 'Send to friends', icon: 'heroicons:paper-airplane' },
    { id: 'link', label: 'Link', icon: 'heroicons:link' },
] as const;

// "Start at" of a scene share (seconds)
const startEnabled = ref(false);
const startAt = ref(0);

const reset = () => {
    tab.value = canSend.value ? 'send' : 'link';
    const s = props.subject;
    startAt.value = s.kind === 'scene' ? Math.floor(s.time ?? 0) : 0;
    startEnabled.value = false;
};
watch(
    () => props.visible,
    (v) => {
        if (v) reset();
    },
    { immediate: true },
);

const start = computed(() => (startEnabled.value && startAt.value > 0 ? startAt.value : 0));

const items = computed<ShareTarget[]>(() => {
    const s = props.subject;
    switch (s.kind) {
        case 'scene':
            return [
                {
                    kind: 'scene',
                    scene_id: s.sceneId,
                    ...(start.value ? { time: start.value } : {}),
                },
            ];
        case 'scenes':
            return s.sceneIds.map((id) => ({ kind: 'scene' as const, scene_id: id }));
        case 'marker':
            return [{ kind: 'marker', marker_id: s.markerId }];
        case 'playlist':
            return [{ kind: 'playlist', playlist_uuid: s.uuid }];
        case 'lobby':
            return [{ kind: 'lobby', lobby_id: s.lobbyId }];
    }
    return [];
});

const heading = computed(() => {
    const s = props.subject;
    switch (s.kind) {
        case 'scene':
            return s.title ? `Share "${s.title}"` : 'Share video';
        case 'scenes':
            return `Share ${s.sceneIds.length} video${s.sceneIds.length === 1 ? '' : 's'}`;
        case 'marker':
            return 'Share marker';
        case 'playlist':
            return `Share "${s.name}"`;
        case 'lobby':
            return 'Invite friends';
    }
    return 'Share';
});

const close = () => emit('close');
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 px-4
                backdrop-blur-sm"
            @click.self="close"
        >
            <div class="glass-panel border-border w-full max-w-md border p-5">
                <div class="mb-3 flex items-center justify-between gap-3">
                    <h3 class="truncate text-sm font-semibold text-white">{{ heading }}</h3>
                    <button class="text-dim transition-colors hover:text-white" @click="close">
                        <Icon name="heroicons:x-mark" size="16" />
                    </button>
                </div>

                <div v-if="canSend && canLink" class="border-border mb-3 flex gap-1 border-b">
                    <button
                        v-for="t in tabs"
                        :key="t.id"
                        class="-mb-px flex items-center gap-1.5 border-b-2 px-3 py-1.5 text-xs
                            font-medium transition-colors"
                        :class="
                            tab === t.id
                                ? 'border-lava text-lava'
                                : 'text-dim border-transparent hover:text-white'
                        "
                        @click="tab = t.id"
                    >
                        <Icon :name="t.icon" size="12" />
                        {{ t.label }}
                    </button>
                </div>

                <ShareSubjectSummary :subject="subject" class="mb-3" />
                <ShareTimecodeField
                    v-if="subject.kind === 'scene'"
                    v-model:enabled="startEnabled"
                    v-model:seconds="startAt"
                    :max="subject.duration"
                    class="mb-3"
                />

                <ShareSendPanel
                    v-if="tab === 'send' && canSend"
                    :items="items"
                    :lobby="subject.kind === 'lobby'"
                    @sent="close"
                />
                <ShareLinkPanel
                    v-else-if="canLink && (subject.kind === 'scene' || subject.kind === 'marker')"
                    :scene-id="subject.sceneId"
                    :start="start"
                    :marker-id="subject.kind === 'marker' ? subject.markerId : undefined"
                />
                <p v-else class="text-dim py-4 text-center text-xs">
                    You are not allowed to share this.
                </p>
            </div>
        </div>
    </Teleport>
</template>
