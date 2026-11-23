<script setup lang="ts">
import type { SceneCopyBulkResult, SceneCopyItem } from '~/types/duplicates';

// Permanent deletion of copy files from disk: one copy, or the selected copies. The only
// destructive action of the copy triage: the user must tick "I understand". The server checks
// each copy again (read-only storage path, same content as the scene's main file, never the
// main file) and refuses otherwise.
const props = defineProps<{
    visible: boolean;
    // Copies loaded on the client, shown with path and size
    copies: SceneCopyItem[];
    // Every copy ID to delete (may include copies not loaded, selected on other pages)
    ids: number[];
}>();

const emit = defineEmits<{
    close: [];
    deleted: [message: string, result: SceneCopyBulkResult];
}>();

const { deleteCopyFile, deleteCopyFilesBulk } = useApiDuplicates();
const { formatSize } = useFormatter();

const understood = ref(false);
const busy = ref(false);
const error = ref<string | null>(null);

watch(
    () => props.visible,
    (v) => {
        if (v) {
            understood.value = false;
            error.value = null;
        }
    },
);

const knownSize = computed(() => props.copies.reduce((sum, c) => sum + c.size, 0));
const unknownCount = computed(() => Math.max(0, props.ids.length - props.copies.length));
const single = computed(() => props.ids.length === 1);

const handleEscape = (e: KeyboardEvent) => {
    if (props.visible && e.key === 'Escape' && !busy.value) emit('close');
};
onMounted(() => window.addEventListener('keydown', handleEscape));
onUnmounted(() => window.removeEventListener('keydown', handleEscape));

const confirm = async () => {
    if (!understood.value || busy.value || props.ids.length === 0) return;
    busy.value = true;
    error.value = null;
    try {
        let result: SceneCopyBulkResult;
        if (single.value) {
            const id = props.ids[0]!;
            const res = await deleteCopyFile(id);
            result = { done: [id], failed: [], freed_bytes: res.freed_bytes };
        } else {
            result = await deleteCopyFilesBulk(props.ids);
        }
        emit('deleted', describeCopyBulkResult('delete', result, formatSize), result);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Deletion failed';
    } finally {
        busy.value = false;
    }
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
            @click.self="!busy && emit('close')"
        >
            <div
                class="border-lava/30 bg-panel mx-4 w-full max-w-lg rounded-xl border p-5
                    shadow-2xl"
            >
                <div class="mb-3 flex items-start gap-3">
                    <div class="bg-lava/10 flex items-center justify-center rounded-full p-2">
                        <Icon name="heroicons:trash" size="20" class="text-lava" />
                    </div>
                    <div class="min-w-0">
                        <h3 class="text-sm font-medium text-white">
                            Delete {{ single ? 'this file' : `${ids.length} files` }} from disk
                        </h3>
                        <p class="text-lava mt-1 text-xs">
                            The file{{ single ? ' is' : 's are' }} permanently deleted from disk and
                            cannot be recovered.
                        </p>
                    </div>
                </div>

                <ul
                    class="border-border mb-3 max-h-48 space-y-1 overflow-y-auto rounded-md border
                        bg-black/20 p-2"
                >
                    <li v-for="c in copies" :key="c.id" class="font-mono text-[10px]">
                        <span class="break-all text-white/80">{{ c.stored_path }}</span>
                        <span class="text-dim"> · {{ formatSize(c.size) }}</span>
                    </li>
                    <li v-if="unknownCount > 0" class="text-dim text-[10px]">
                        and {{ unknownCount }} more selected on other pages
                    </li>
                </ul>
                <p class="text-dim mb-1 text-xs">
                    Total: <span class="text-white">{{ formatSize(knownSize) }}</span
                    >{{ unknownCount > 0 ? ' (plus the copies not loaded)' : '' }}.
                </p>
                <p class="text-dim mb-3 text-xs">
                    Before deleting, each copy and its scene's main file are read again: a copy is
                    kept when its content no longer matches the main file, when the main file cannot
                    be read, or when it is on a read-only storage path. The scene's main file is
                    never deleted here.
                </p>

                <label class="mb-4 flex items-center gap-2 text-xs text-white">
                    <input v-model="understood" type="checkbox" class="accent-lava" />
                    I understand that the file{{ single ? '' : 's' }} will be deleted from disk and
                    cannot be recovered
                </label>

                <p v-if="error" class="text-lava mb-3 text-xs">{{ error }}</p>

                <div class="flex justify-end gap-2">
                    <button
                        class="border-border hover:bg-surface-hover rounded-lg border px-3 py-1.5
                            text-xs text-white transition-colors"
                        :disabled="busy"
                        @click="emit('close')"
                    >
                        Cancel
                    </button>
                    <button
                        class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-xs
                            font-semibold text-white transition-colors disabled:opacity-40"
                        :disabled="!understood || busy"
                        @click="confirm"
                    >
                        {{ busy ? 'Deleting...' : 'Delete from disk' }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
