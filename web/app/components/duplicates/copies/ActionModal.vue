<script setup lang="ts">
import type { SceneCopyItem, SceneWithCopies } from '~/types/duplicates';

// Confirmation of an action on one copy that never touches a file: make it the main file,
// separate it into its own scene, or forget it.
const props = defineProps<{
    visible: boolean;
    action: 'make-main' | 'separate' | 'forget' | null;
    copy: SceneCopyItem | null;
    scene: SceneWithCopies | null;
}>();

const emit = defineEmits<{
    close: [];
    done: [message: string];
}>();

const { copyAction } = useApiDuplicates();
const { formatSize } = useFormatter();

const busy = ref(false);
const error = ref<string | null>(null);

watch(
    () => props.visible,
    (v) => {
        if (v) error.value = null;
    },
);

const texts = computed(() => {
    switch (props.action) {
        case 'make-main':
            return {
                title: 'Make main file',
                confirm: 'Make main file',
                icon: 'heroicons:star',
            };
        case 'separate':
            return {
                title: 'Separate scene',
                confirm: 'Create scene',
                icon: 'heroicons:square-2-stack',
            };
        default:
            return { title: 'Forget copy', confirm: 'Forget', icon: 'heroicons:eye-slash' };
    }
});

const confirm = async () => {
    if (!props.copy || !props.action || busy.value) return;
    busy.value = true;
    error.value = null;
    try {
        const res = (await copyAction(props.copy.id, props.action)) as { scene_id?: number };
        const name = fileName(props.copy.stored_path);
        const msg =
            props.action === 'make-main'
                ? `${name} is now the main file of the scene`
                : props.action === 'separate'
                  ? `${name} is now scene #${res?.scene_id ?? '?'} (processing queued)`
                  : `${name} forgotten (the file stays on disk)`;
        emit('done', msg);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Action failed';
    } finally {
        busy.value = false;
    }
};
</script>

<template>
    <UiConfirmModal
        :visible="visible && !!copy && !!action"
        :title="texts.title"
        :confirm-label="busy ? 'Working...' : texts.confirm"
        :icon="texts.icon"
        @close="emit('close')"
        @confirm="confirm"
    >
        <template v-if="copy">
            <p class="font-mono text-[10px] break-all text-white/80">{{ copy.stored_path }}</p>
            <p class="font-mono text-[10px]">{{ formatSize(copy.size) }}</p>
            <p v-if="action === 'make-main'">
                The scene keeps its ID, markers, tags and history; its main file becomes this path,
                and the current main file
                <span class="font-mono text-[10px] break-all text-white/70">{{
                    scene?.stored_path
                }}</span>
                becomes a copy. No file is moved or deleted.
            </p>
            <p v-else-if="action === 'separate'">
                This file becomes a scene of its own (title from the file name, processing queued),
                no longer linked to "{{ scene?.title }}". No file is moved or deleted.
            </p>
            <p v-else>
                The copy is removed from this list only. The file stays on disk, and the next scan
                records it again as a copy.
            </p>
            <p v-if="error" class="text-lava">{{ error }}</p>
        </template>
    </UiConfirmModal>
</template>
