<script setup lang="ts">
import type { ShareResult, ShareTarget } from '~/types/messages';

// "Send to friends" of the Share dialog: pick users, an optional message, send (one message
// per item and user, the text on the first one). Tells who could not be reached.
const props = defineProps<{ items: ShareTarget[]; lobby?: boolean }>();
const emit = defineEmits<{ sent: [] }>();

const api = useApiMessages();
const selected = ref<number[]>([]);
const text = ref('');
const sending = ref(false);
const error = ref('');
const failures = ref<ShareResult[]>([]);
const done = ref(0);

async function send() {
    if (selected.value.length === 0 || sending.value) return;
    sending.value = true;
    error.value = '';
    failures.value = [];
    try {
        const res = await api.share(selected.value, props.items, text.value.trim());
        failures.value = res.results.filter((r) => !r.ok);
        done.value = res.results.length - failures.value.length;
        if (failures.value.length === 0) {
            setTimeout(() => emit('sent'), 700);
        } else {
            selected.value = failures.value.map((f) => f.user_id);
        }
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to send';
    } finally {
        sending.value = false;
    }
}
</script>

<template>
    <div class="space-y-3">
        <ShareRecipientPicker v-model="selected" />
        <textarea
            v-model="text"
            rows="2"
            maxlength="2000"
            :placeholder="lobby ? 'Add a message (optional)' : 'Say something about it (optional)'"
            class="border-border bg-surface focus:border-lava/40 w-full resize-none rounded-md
                border px-2.5 py-1.5 text-xs text-white outline-none placeholder:text-white/30"
        />
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-[11px]"
        >
            {{ error }}
        </div>
        <div
            v-if="failures.length > 0"
            class="rounded-lg border border-amber-500/20 bg-amber-500/5 px-3 py-2 text-[11px]
                text-amber-300"
        >
            <div v-if="done > 0">Sent to {{ done }}. Not sent to the others:</div>
            <div v-for="f in failures" :key="f.user_id">{{ f.error || 'Failed' }}</div>
        </div>
        <div
            v-else-if="done > 0"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2
                text-[11px]"
        >
            Sent to {{ done }} {{ done === 1 ? 'person' : 'people' }}.
        </div>
        <button
            class="bg-lava hover:bg-lava-glow flex w-full items-center justify-center gap-1.5
                rounded-lg px-4 py-2 text-xs font-semibold text-white transition-all
                disabled:cursor-not-allowed disabled:opacity-40"
            :disabled="selected.length === 0 || sending || items.length === 0"
            @click="send"
        >
            <Icon name="heroicons:paper-airplane" size="13" />
            {{
                sending
                    ? 'Sending...'
                    : selected.length > 1
                      ? `Send to ${selected.length} people`
                      : 'Send'
            }}
        </button>
    </div>
</template>
