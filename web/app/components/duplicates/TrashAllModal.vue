<script setup lang="ts">
import type { BulkTrashAllResult, DuplicateGroup, TrashAllResult } from '~/types/duplicates';

// Confirms moving every video of one group (or of the selected groups, `bulk`) to the trash,
// the best one included, then resolves the groups. Groups selected on another page are not
// loaded here: they are counted apart from the videos and size.
const props = defineProps<{
    visible: boolean;
    groupIds: number[];
    groups: ReadonlyMap<number, DuplicateGroup>;
    bulk: boolean;
}>();

const emit = defineEmits<{
    close: [];
    trashed: [result: TrashAllResult | BulkTrashAllResult];
}>();

const { trashAll, trashAllBulk } = useApiDuplicates();
const { formatSize } = useFormatter();

const loading = ref(false);
const error = ref<string | null>(null);

const summary = computed(() => summarizeTrashAll(props.groupIds, props.groups));

watch(
    () => props.visible,
    (visible) => {
        if (visible) error.value = null;
    },
);

const handleConfirm = async () => {
    if (props.groupIds.length === 0) return;
    loading.value = true;
    error.value = null;
    try {
        const result =
            props.bulk || props.groupIds.length > 1
                ? await trashAllBulk(props.groupIds)
                : await trashAll(props.groupIds[0]!);
        emit('trashed', result);
        emit('close');
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to move the videos to the trash';
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    if (!loading.value) emit('close');
};

const handleEscape = (e: KeyboardEvent) => {
    if (props.visible && e.key === 'Escape') handleClose();
};

onMounted(() => window.addEventListener('keydown', handleEscape));
onUnmounted(() => window.removeEventListener('keydown', handleEscape));
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div class="border-border bg-panel mx-4 w-full max-w-sm rounded-xl border shadow-2xl">
                <div class="border-border flex items-center justify-between border-b px-4 py-3">
                    <h2 class="text-sm font-semibold text-white">
                        {{ bulk ? 'Trash all videos of the selected groups' : 'Trash all videos' }}
                    </h2>
                    <button
                        class="text-dim transition-colors hover:text-white"
                        @click="handleClose"
                    >
                        <Icon name="heroicons:x-mark" size="18" />
                    </button>
                </div>

                <div class="p-4">
                    <div class="mb-4 flex items-start gap-3">
                        <div
                            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg
                                bg-red-500/10"
                        >
                            <Icon name="heroicons:trash" size="20" class="text-red-400" />
                        </div>
                        <div>
                            <p class="text-sm font-medium text-white">
                                Move {{ summary.videos }} video{{
                                    summary.videos === 1 ? '' : 's'
                                }}
                                ({{
                                    summary.size > 0 || summary.unknownSize === 0
                                        ? formatSize(summary.size)
                                        : 'size unknown'
                                }}) to the trash?
                            </p>
                            <p class="text-dim mt-1 text-xs">
                                Every video of
                                {{
                                    bulk
                                        ? `the ${groupIds.length} selected group${groupIds.length === 1 ? '' : 's'}`
                                        : 'this group'
                                }}
                                goes to the trash, the best one included, and the group is marked
                                resolved with no kept video. They can be restored from the trash
                                until its retention period ends. No file is deleted now.
                            </p>
                            <p
                                v-if="summary.unknownSize && summary.size > 0"
                                class="text-dim mt-1 text-xs"
                            >
                                Size of {{ summary.unknownSize }} video(s) unknown.
                            </p>
                            <p v-if="summary.alreadyTrashed" class="text-dim mt-1 text-xs">
                                {{ summary.alreadyTrashed }} video(s) already in the trash are left
                                as they are.
                            </p>
                            <p v-if="summary.missingGroups" class="text-dim mt-1 text-xs">
                                {{ summary.missingGroups }} selected group(s) are on another page:
                                their videos are not counted above but are trashed too.
                            </p>
                        </div>
                    </div>

                    <ErrorAlert v-if="error" :message="error" />
                </div>

                <div class="border-border flex items-center justify-end gap-2 border-t px-4 py-3">
                    <button
                        :disabled="loading"
                        class="border-border hover:border-border-hover rounded-lg border px-3 py-1.5
                            text-xs font-medium text-white transition-all disabled:opacity-50"
                        @click="handleClose"
                    >
                        Cancel
                    </button>
                    <button
                        :disabled="loading || groupIds.length === 0"
                        class="flex items-center gap-1 rounded-lg bg-red-600 px-3 py-1.5 text-xs
                            font-semibold text-white transition-colors hover:bg-red-500
                            disabled:opacity-50"
                        @click="handleConfirm"
                    >
                        <Icon
                            v-if="loading"
                            name="heroicons:arrow-path"
                            size="14"
                            class="animate-spin"
                        />
                        <Icon v-else name="heroicons:trash" size="14" />
                        {{ loading ? 'Moving to trash...' : 'Move all to trash' }}
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>
