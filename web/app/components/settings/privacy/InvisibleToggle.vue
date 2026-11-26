<script setup lang="ts">
// Invisible mode (presence): appear offline to everyone. Saved at once.
const emit = defineEmits<{ saved: [message: string]; failed: [message: string] }>();

const presenceStore = usePresenceStore();
const invisible = ref(false);
const loading = ref(true);
const saving = ref(false);

onMounted(async () => {
    try {
        invisible.value = (await useApiPresence().getPresenceSettings()).invisible;
    } catch {
        // stays visible in the form; saving tells if the server is reachable
    } finally {
        loading.value = false;
    }
});

async function set(value: boolean) {
    const previous = invisible.value;
    invisible.value = value;
    saving.value = true;
    try {
        await presenceStore.setInvisible(value);
        emit('saved', value ? 'You now appear offline' : 'Your status is visible again');
    } catch (e: unknown) {
        invisible.value = previous;
        emit('failed', e instanceof Error ? e.message : 'Failed to save the invisible mode');
    } finally {
        saving.value = false;
    }
}
</script>

<template>
    <div class="flex items-center justify-between gap-4 py-2.5">
        <div class="min-w-0">
            <div class="text-xs text-white">Invisible</div>
            <div class="text-dim text-[11px]">
                Appear offline to everyone, whatever the audiences below. What you watch is not
                shown either, and your "last seen" stays at the moment you turned it on.
            </div>
        </div>
        <UiToggle
            :model-value="invisible"
            :disabled="loading || saving"
            @update:model-value="set"
        />
    </div>
</template>
