<script setup lang="ts">
import type { MessagePolicy, MessageSettings } from '~/types/messages';

// Messages settings (Settings > Privacy): who may write to me, and the toasts of incoming
// messages. Saved at once.
const emit = defineEmits<{ saved: [message: string]; failed: [message: string] }>();

const api = useApiMessages();
const messagesStore = useMessagesStore();
const settings = ref<MessageSettings | null>(null);
const saving = ref(false);

const options = computed(() =>
    (settings.value?.policies ?? []).map((p) => ({ value: p.value, label: p.label })),
);
const description = computed(
    () =>
        settings.value?.policies.find((p) => p.value === settings.value?.policy)?.description ?? '',
);

onMounted(async () => {
    try {
        settings.value = await api.getSettings();
    } catch (e: unknown) {
        emit('failed', e instanceof Error ? e.message : 'Failed to load the message settings');
    }
});

async function save(update: { policy?: MessagePolicy; toasts?: boolean }, text: string) {
    saving.value = true;
    try {
        settings.value = await api.updateSettings(update);
        messagesStore.settings = settings.value;
        emit('saved', text);
    } catch (e: unknown) {
        emit('failed', e instanceof Error ? e.message : 'Failed to save the message settings');
    } finally {
        saving.value = false;
    }
}

const setPolicy = (value: string | number) =>
    save({ policy: String(value) as MessagePolicy }, 'Message settings saved');
const setToasts = (value: boolean) =>
    save({ toasts: value }, value ? 'Message toasts on' : 'Message toasts off');
</script>

<template>
    <div class="glass-panel p-5">
        <h3 class="mb-1 text-sm font-semibold text-white">Messages</h3>
        <p class="text-dim mb-3 text-[11px]">
            Who can write to you and share videos with you. It applies when a message is sent: if
            someone no longer passes it (for example you are no longer friends), your conversation
            stays, read-only for them. Administrators cannot read your messages.
        </p>
        <div v-if="!settings" class="flex justify-center py-4"><LoadingSpinner /></div>
        <template v-else>
            <div class="sm:w-64" :class="saving ? 'pointer-events-none opacity-50' : ''">
                <UiSelectMenu
                    :model-value="settings.policy"
                    :options="options"
                    @update:model-value="setPolicy"
                />
            </div>
            <p class="text-dim mt-2 text-[11px]">{{ description }}</p>
            <div class="border-border mt-3 flex items-center justify-between gap-4 border-t pt-3">
                <div class="min-w-0">
                    <div class="text-xs text-white">Toasts for new messages</div>
                    <div class="text-dim text-[11px]">
                        A small notice when a message arrives outside the Messages page. Muted
                        conversations never show one.
                    </div>
                </div>
                <UiToggle
                    :model-value="settings.toasts"
                    :disabled="saving"
                    @update:model-value="setToasts"
                />
            </div>
        </template>
    </div>
</template>
