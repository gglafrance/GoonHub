<script setup lang="ts">
import type { DuplicateGroup } from '~/types/duplicates';

// Confirms removing the unchecked members of a group (not duplicates of the others) from it,
// and with `resolve`, resolving the group right after on its eligible members only. The removed
// videos are recorded as not duplicates of the remaining ones (fingerprint matching won't
// regroup them) and are never trashed nor deleted.
const props = defineProps<{
    visible: boolean;
    group: DuplicateGroup | null;
    unchecked: ReadonlySet<number>;
    resolve: boolean;
    mergeMetadata: boolean;
}>();

const emit = defineEmits<{
    close: [];
    done: [message: string];
}>();

const { excludeMembers, resolveGroup } = useApiDuplicates();

const loading = ref(false);
const error = ref<string | null>(null);

const removed = computed(() =>
    props.group ? props.group.members.filter((m) => props.unchecked.has(m.scene_id)) : [],
);
const remaining = computed(() => (props.group?.members.length ?? 0) - removed.value.length);
const shownRemoved = computed(() => removed.value.slice(0, 5));
const best = computed(() =>
    props.group?.members.find((m) => m.scene_id === props.group?.best_scene_id),
);

watch(
    () => props.visible,
    (visible) => {
        if (visible) error.value = null;
    },
);

const handleConfirm = async () => {
    const group = props.group;
    if (!group || removed.value.length === 0 || loading.value) return;
    loading.value = true;
    error.value = null;
    try {
        const result = await excludeMembers(
            group.id,
            removed.value.map((m) => m.scene_id),
        );
        let message =
            `${result.excluded.length} video(s) removed from group #${group.id}` +
            (result.dismissed ? ', the group is dismissed (one video left)' : '');
        if (props.resolve && !result.dismissed && result.group.best_scene_id) {
            const resolved = await resolveGroup(
                group.id,
                result.group.best_scene_id,
                props.mergeMetadata,
            );
            const markers = describeMarkerMerge(resolved.markers);
            message += `, then resolved` + (markers ? `. Markers: ${markers}` : '');
        }
        emit('done', message);
        emit('close');
    } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to remove the videos from the group';
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    if (!loading.value) emit('close');
};
</script>

<template>
    <UiConfirmModal
        :visible="visible && group !== null"
        :title="resolve ? 'Resolve with the eligible videos only' : 'Remove unchecked videos'"
        :confirm-label="
            loading ? 'Working...' : resolve ? 'Remove and resolve' : 'Remove from group'
        "
        icon="heroicons:minus-circle"
        @close="handleClose"
        @confirm="handleConfirm"
    >
        <p>
            {{ removed.length }} unchecked video{{ removed.length === 1 ? '' : 's' }} will be
            removed from group #{{ group?.id }} and remembered as not duplicates of the
            {{ remaining }} other{{ remaining === 1 ? '' : 's' }}, so a later fingerprint run won't
            group them again. They are not trashed nor deleted.
        </p>
        <ul class="font-mono text-[10px]">
            <li v-for="m in shownRemoved" :key="m.scene_id" class="truncate">
                #{{ m.scene_id }} {{ m.title }}
            </li>
            <li v-if="removed.length > shownRemoved.length">
                and {{ removed.length - shownRemoved.length }} more
            </li>
        </ul>
        <p v-if="resolve && best">
            Then the group is resolved: #{{ best.scene_id }} {{ best.title }} is kept and the other
            {{ remaining - 1 }} eligible video{{ remaining - 1 === 1 ? '' : 's' }} go to the trash{{
                mergeMetadata ? ' after their metadata is merged into it' : ''
            }}.
        </p>
        <p v-else-if="remaining < 2" class="text-lava">
            Only one video is left: the group is dismissed.
        </p>
        <ErrorAlert v-if="error" :message="error" />
    </UiConfirmModal>
</template>
