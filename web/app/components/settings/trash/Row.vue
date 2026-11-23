<script setup lang="ts">
// One trashed video of the trash list: selection, file, specs, dates and row actions.
import type { TrashedScene } from '~/types/trash';

const props = defineProps<{
    scene: TrashedScene;
    selected: boolean;
    now: number;
    busy: boolean;
}>();

const emit = defineEmits<{
    toggle: [];
    restore: [];
    delete: [];
}>();

const { formatSize, formatDuration } = useFormatter();
const { formatDateTime } = useDateFormatter();

const thumbFailed = ref(false);
const purge = computed(() => purgeCountdown(props.scene.expires_at, props.now));
const specs = computed(() =>
    [
        props.scene.duration ? formatDuration(props.scene.duration) : '',
        resolutionLabel(props.scene.height),
        props.scene.video_codec,
    ].filter(Boolean),
);
</script>

<template>
    <div
        class="flex items-center gap-3 rounded-lg border p-2.5 transition-colors"
        :class="
            selected
                ? 'border-lava/40 bg-lava/5'
                : 'border-border/50 bg-surface/30 hover:border-white/15'
        "
    >
        <input
            type="checkbox"
            class="accent-lava h-3.5 w-3.5 shrink-0 cursor-pointer"
            :checked="selected"
            :aria-label="`Select ${scene.title}`"
            @change="emit('toggle')"
        />

        <div class="relative h-14 w-24 shrink-0 overflow-hidden rounded bg-black">
            <template v-if="scene.thumbnail_path && !thumbFailed">
                <img
                    :src="`/thumbnails/${scene.id}`"
                    class="absolute inset-0 h-full w-full scale-110 object-cover blur-md"
                    alt=""
                    aria-hidden="true"
                    loading="lazy"
                />
                <img
                    :src="`/thumbnails/${scene.id}`"
                    :alt="scene.title"
                    class="absolute inset-0 h-full w-full object-contain"
                    loading="lazy"
                    @error="thumbFailed = true"
                />
            </template>
            <div v-else class="flex h-full items-center justify-center">
                <Icon name="heroicons:film" size="20" class="text-dim/50" />
            </div>
        </div>

        <div class="min-w-0 flex-1">
            <div class="flex items-center gap-1.5">
                <h4 class="truncate text-[13px] font-medium text-white" :title="scene.title">
                    {{ scene.title || `Scene #${scene.id}` }}
                </h4>
                <span
                    v-if="scene.source === 'duplicates'"
                    class="shrink-0 rounded-full border border-white/10 bg-white/5 px-1.5
                        text-[10px] text-white/70"
                    :title="`Not kept when duplicate group #${scene.duplicate_group_id} was resolved`"
                >
                    Duplicate
                </span>
                <span
                    v-if="scene.kept_on_disk"
                    class="border-emerald/30 bg-emerald/10 text-emerald shrink-0 rounded-full border
                        px-1.5 text-[10px]"
                    title="Read-only storage path: a permanent deletion keeps the file on disk"
                >
                    Kept on disk
                </span>
                <span
                    v-if="scene.file_missing"
                    class="shrink-0 rounded-full border border-amber-500/30 bg-amber-500/10 px-1.5
                        text-[10px] text-amber-500"
                    title="The last missing-file check did not find this file"
                >
                    File missing
                </span>
            </div>
            <p class="text-dim mt-0.5 truncate font-mono text-[10px]" :title="scene.stored_path">
                <span v-if="scene.storage_path_name" class="text-white/60">
                    {{ scene.storage_path_name }}:
                </span>
                {{ scene.relative_path }}
            </p>
            <p class="text-dim mt-0.5 flex flex-wrap items-center gap-x-2 text-[11px]">
                <span class="font-medium text-white/80">{{
                    scene.size ? formatSize(scene.size) : 'Unknown size'
                }}</span>
                <span v-for="spec in specs" :key="spec">{{ spec }}</span>
                <span>Trashed <UiFormattedDate :datetime="scene.trashed_at" time /></span>
            </p>
        </div>

        <div
            class="shrink-0 rounded-full border px-2 py-0.5 text-[10px] font-medium"
            :class="
                purge.soon
                    ? 'border-amber-500/30 bg-amber-500/15 text-amber-500'
                    : 'bg-surface/50 text-dim border-border/50'
            "
            :title="`Purged on ${formatDateTime(scene.expires_at)}${
                scene.scheduled_deletion_at ? ' (scheduled duplicate deletion)' : ''
            }`"
        >
            {{ purge.label }}
        </div>

        <div class="flex shrink-0 gap-1">
            <button
                class="text-dim hover:text-emerald rounded p-1.5 transition-colors
                    disabled:opacity-40"
                title="Restore"
                :disabled="busy"
                @click="emit('restore')"
            >
                <Icon name="heroicons:arrow-uturn-left" size="14" />
            </button>
            <button
                class="text-dim hover:text-lava rounded p-1.5 transition-colors disabled:opacity-40"
                title="Delete permanently"
                :disabled="busy"
                @click="emit('delete')"
            >
                <Icon name="heroicons:trash" size="14" />
            </button>
        </div>
    </div>
</template>
