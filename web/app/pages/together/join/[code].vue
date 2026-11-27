<script setup lang="ts">
// Invite link (/together/join/<code>): joins the lobby of the code, then opens it.
useHead({ title: 'Join lobby' });

const route = useRoute();
const { joinByCode } = useApiTogether();

const error = ref('');

onMounted(async () => {
    const code = String(route.params.code ?? '');
    try {
        const lobby = await joinByCode(code);
        await navigateTo(`/together/${lobby.id}`, { replace: true });
    } catch (e: unknown) {
        const status = (e as { status?: number }).status;
        error.value =
            status === 404
                ? 'This invite link is not valid anymore: the lobby was closed or its code changed.'
                : status === 403
                  ? 'You are banned from this lobby, or not allowed to join lobbies.'
                  : e instanceof Error
                    ? e.message
                    : 'Failed to join the lobby';
    }
});

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="flex min-h-[70vh] flex-col items-center justify-center p-4 text-center">
        <template v-if="!error">
            <LoadingSpinner label="Joining the lobby..." />
        </template>
        <template v-else>
            <div
                class="border-lava/20 bg-lava/5 flex h-12 w-12 items-center justify-center
                    rounded-xl border"
            >
                <Icon name="heroicons:link-slash" size="24" class="text-lava" />
            </div>
            <h2 class="mt-4 text-lg font-semibold text-white">Cannot join</h2>
            <p class="text-dim mt-1 max-w-sm text-xs">{{ error }}</p>
            <NuxtLink
                to="/together"
                class="border-border bg-surface text-muted hover:border-border-hover mt-6 rounded-lg
                    border px-4 py-2 text-xs font-medium transition-all hover:text-white"
            >
                Browse lobbies
            </NuxtLink>
        </template>
    </div>
</template>
