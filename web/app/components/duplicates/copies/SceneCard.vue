<script setup lang="ts">
import type { CopyAction, SceneCopyItem, SceneWithCopies } from '~/types/duplicates';
import type { StoragePath } from '~/types/storage';

// A scene with its main file and its copies (those matching the filters), with a checkbox and
// the actions of each copy.
const props = defineProps<{
    scene: SceneWithCopies;
    storagePaths: ReadonlyMap<number, StoragePath>;
    selected: ReadonlySet<number>;
}>();

const emit = defineEmits<{
    toggle: [copyId: number];
    toggleAll: [copyIds: number[], select: boolean];
    action: [action: CopyAction, copy: SceneCopyItem];
}>();

const { formatSize } = useFormatter();

const spName = (id: number | null) => (id ? (props.storagePaths.get(id)?.name ?? `#${id}`) : '-');
const isReadOnly = (id: number | null) => (id ? !!props.storagePaths.get(id)?.read_only : false);

const allSelected = computed(
    () =>
        props.scene.copies.length > 0 && props.scene.copies.every((c) => props.selected.has(c.id)),
);
const toggleAll = () => {
    emit(
        'toggleAll',
        props.scene.copies.map((c) => c.id),
        !allSelected.value,
    );
};
</script>

<template>
    <div class="border-border rounded-lg border bg-white/2 p-3">
        <!-- Main file -->
        <div class="flex items-start gap-3">
            <input
                type="checkbox"
                class="accent-lava mt-1"
                :checked="allSelected"
                title="Select the copies of this scene"
                @change="toggleAll"
            />
            <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                    <NuxtLink
                        :to="`/watch/${scene.scene_id}`"
                        class="hover:text-lava truncate text-xs font-medium text-white
                            transition-colors"
                    >
                        {{ scene.title || fileName(scene.stored_path) }}
                    </NuxtLink>
                    <span
                        v-if="scene.is_trashed"
                        class="rounded border border-amber-500/30 bg-amber-500/10 px-1 text-[9px]
                            text-amber-400"
                        >In trash</span
                    >
                    <span
                        v-if="scene.is_missing"
                        class="border-lava/30 bg-lava/10 text-lava rounded border px-1 text-[9px]"
                        title="A scan did not find the main file"
                        >Main file missing</span
                    >
                    <span class="text-dim ml-auto font-mono text-[10px]">
                        {{ scene.copy_count }} cop{{ scene.copy_count === 1 ? 'y' : 'ies' }} ·
                        {{ formatSize(scene.copies_size) }}
                    </span>
                </div>
                <p class="text-dim/70 mt-0.5 font-mono text-[10px] break-all">
                    <span class="text-dim/50">Main</span> {{ scene.stored_path }}
                </p>
                <p class="text-dim/40 font-mono text-[10px]">
                    {{ formatSize(scene.size) }} · {{ spName(scene.storage_path_id) }}
                </p>
            </div>
        </div>

        <!-- Copies -->
        <ul class="border-border mt-2 ml-6 space-y-1.5 border-l pl-3">
            <li v-for="copy in scene.copies" :key="copy.id" class="flex items-start gap-2">
                <input
                    type="checkbox"
                    class="accent-lava mt-0.5"
                    :checked="selected.has(copy.id)"
                    @change="emit('toggle', copy.id)"
                />
                <div class="min-w-0 flex-1">
                    <p class="font-mono text-[10px] break-all text-white/80">
                        {{ copy.stored_path }}
                    </p>
                    <p
                        class="text-dim/50 flex flex-wrap items-center gap-x-2 font-mono
                            text-[10px]"
                    >
                        <span>{{ formatSize(copy.size) }}</span>
                        <span>
                            {{ spName(copy.storage_path_id) }}
                            <span v-if="isReadOnly(copy.storage_path_id)" class="text-amber-400/80"
                                >(read-only)</span
                            >
                        </span>
                        <span>seen <NuxtTime :datetime="copy.last_seen_at" relative /></span>
                        <span
                            v-if="copy.not_seen_in_last_scan"
                            class="text-amber-400/80"
                            title="The last completed scan did not see this file"
                            >not seen in the last scan</span
                        >
                    </p>
                </div>
                <div class="flex shrink-0 items-center gap-0.5">
                    <button
                        class="text-dim rounded p-1 transition-colors hover:bg-white/5
                            hover:text-white"
                        title="Make main file"
                        @click="emit('action', 'make-main', copy)"
                    >
                        <Icon name="heroicons:star" size="13" />
                    </button>
                    <button
                        class="text-dim rounded p-1 transition-colors hover:bg-white/5
                            hover:text-white"
                        title="Separate scene"
                        @click="emit('action', 'separate', copy)"
                    >
                        <Icon name="heroicons:square-2-stack" size="13" />
                    </button>
                    <button
                        class="text-dim rounded p-1 transition-colors hover:bg-white/5
                            hover:text-white"
                        title="Forget (the file stays on disk)"
                        @click="emit('action', 'forget', copy)"
                    >
                        <Icon name="heroicons:eye-slash" size="13" />
                    </button>
                    <button
                        class="text-dim hover:text-lava rounded p-1 transition-colors
                            hover:bg-white/5 disabled:opacity-30"
                        :disabled="isReadOnly(copy.storage_path_id)"
                        :title="
                            isReadOnly(copy.storage_path_id)
                                ? 'Read-only storage path: the file cannot be deleted'
                                : 'Delete file from disk'
                        "
                        @click="emit('action', 'delete', copy)"
                    >
                        <Icon name="heroicons:trash" size="13" />
                    </button>
                </div>
            </li>
        </ul>
    </div>
</template>
