<script setup lang="ts">
import type { Tag } from '~/types/tag';

// Deletes several tags (the selection of the tag list). A tag is removed from its videos;
// the videos themselves are never deleted. Each tag goes through the single delete endpoint,
// one after the other, so a failure only leaves that tag.
const props = defineProps<{
    visible: boolean;
    tags: Tag[];
}>();

const emit = defineEmits<{
    close: [];
    deleted: [ids: number[]];
}>();

const { deleteTag } = useApiTags();

const running = ref(false);
const done = ref(0);
const failures = ref<{ name: string; error: string }[]>([]);

const sceneLinks = computed(() => props.tags.reduce((sum, t) => sum + (t.scene_count || 0), 0));

watch(
    () => props.visible,
    (v) => {
        if (v) {
            done.value = 0;
            failures.value = [];
        }
    },
);

async function confirm() {
    running.value = true;
    failures.value = [];
    done.value = 0;
    const deleted: number[] = [];
    for (const tag of props.tags) {
        try {
            await deleteTag(tag.id);
            deleted.push(tag.id);
        } catch (e: unknown) {
            failures.value.push({
                name: tag.name,
                error: e instanceof Error ? e.message : 'Failed to delete the tag',
            });
        }
        done.value++;
    }
    running.value = false;
    if (deleted.length > 0) emit('deleted', deleted);
    if (failures.value.length === 0) emit('close');
}
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4
                backdrop-blur-sm"
            @click.self="!running && emit('close')"
        >
            <div class="glass-panel w-full max-w-lg p-5">
                <h3 class="mb-3 text-sm font-semibold text-white">
                    Delete {{ plural(tags.length, 'tag') }}
                </h3>
                <p class="text-dim mb-3 text-xs">
                    The tags are removed from
                    <span class="text-white">{{ plural(sceneLinks, 'video') }}</span>
                    (tag links). The videos themselves are not deleted. This cannot be undone.
                </p>
                <div
                    class="border-border bg-void/60 mb-4 max-h-48 overflow-y-auto rounded-lg border
                        px-3 py-2"
                >
                    <div
                        v-for="tag in tags"
                        :key="tag.id"
                        class="flex items-center justify-between py-0.5 text-[11px]"
                    >
                        <span class="truncate text-white">{{ tag.name }}</span>
                        <span class="text-dim ml-3 shrink-0">{{
                            plural(tag.scene_count || 0, 'video')
                        }}</span>
                    </div>
                </div>

                <div v-if="failures.length > 0" class="text-lava mb-3 space-y-0.5 text-[11px]">
                    <p v-for="f in failures" :key="f.name">{{ f.name }}: {{ f.error }}</p>
                </div>

                <div class="flex items-center justify-end gap-2">
                    <span v-if="running" class="text-dim mr-auto text-[11px]">
                        {{ done }} / {{ tags.length }}
                    </span>
                    <button
                        class="border-border text-dim rounded-lg border px-3 py-1.5 text-xs
                            transition-colors hover:text-white disabled:opacity-40"
                        :disabled="running"
                        @click="emit('close')"
                    >
                        {{ failures.length > 0 ? 'Close' : 'Cancel' }}
                    </button>
                    <button
                        v-if="failures.length === 0"
                        class="flex items-center gap-1.5 rounded-lg bg-red-600 px-3 py-1.5 text-xs
                            font-semibold text-white transition-colors hover:bg-red-500
                            disabled:opacity-40"
                        :disabled="running || tags.length === 0"
                        @click="confirm"
                    >
                        <Icon v-if="running" name="svg-spinners:ring-resize" size="12" />
                        <Icon v-else name="heroicons:trash" size="12" />
                        Delete {{ plural(tags.length, 'tag') }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
