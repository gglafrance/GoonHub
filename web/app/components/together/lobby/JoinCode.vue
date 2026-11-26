<script setup lang="ts">
// "Join with code": accepts a code or a full invite link.
const { joinByCode } = useApiTogether();

const value = ref('');
const loading = ref(false);
const error = ref('');

const codeOf = (input: string) => {
    const trimmed = input.trim();
    const match = /\/together\/join\/([^/?#\s]+)/.exec(trimmed);
    return decodeURIComponent(match ? match[1]! : trimmed);
};

const submit = async () => {
    const code = codeOf(value.value);
    if (!code || loading.value) return;
    loading.value = true;
    error.value = '';
    try {
        const lobby = await joinByCode(code);
        await navigateTo(`/together/${lobby.id}`);
    } catch (e: unknown) {
        const status = (e as { status?: number }).status;
        error.value =
            status === 404
                ? 'Unknown invite code'
                : status === 403
                  ? 'You are banned from this lobby'
                  : e instanceof Error
                    ? e.message
                    : 'Failed to join';
    } finally {
        loading.value = false;
    }
};
</script>

<template>
    <form class="flex flex-col gap-1" @submit.prevent="submit">
        <div class="flex items-center gap-2">
            <input
                v-model="value"
                type="text"
                placeholder="Invite code or link"
                class="border-border bg-surface focus:border-lava/40 w-48 rounded-lg border px-3
                    py-1.5 font-mono text-xs text-white outline-none sm:w-56"
            />
            <button
                type="submit"
                class="border-border bg-panel hover:border-lava/30 hover:text-lava flex items-center
                    gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium text-white
                    transition-all disabled:opacity-50"
                :disabled="loading || !value.trim()"
            >
                <Icon name="heroicons:key" size="13" />
                Join
            </button>
        </div>
        <p v-if="error" class="text-lava text-[11px]">{{ error }}</p>
    </form>
</template>
