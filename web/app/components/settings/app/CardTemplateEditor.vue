<script setup lang="ts">
import type { SceneCardConfig, BadgeZone, ContentRow } from '~/types/settings';
import type { SceneListItem } from '~/types/scene';

const settingsStore = useSettingsStore();

// Sample scene data for the live preview
const sampleScene: SceneListItem = {
    id: 0,
    title: 'Sample Scene Title',
    duration: 754,
    size: 1_287_000_000,
    thumbnail_path: '',
    preview_video_path: '',
    processing_status: 'completed',
    is_corrupted: false,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    view_count: 42,
    width: 1920,
    height: 1080,
    frame_rate: 60,
    description: 'A sample scene description for preview purposes.',
    studio: 'Sample Studio',
    tags: ['tag-1', 'tag-2', 'tag-3'],
    actors: ['Actor One', 'Actor Two'],
    markers: [
        { id: 1, label: 'Intro', color: '#FFFFFF', timestamp: 12 },
        { id: 2, label: 'Highlight', color: '#FF4D4D', timestamp: 245, end_timestamp: 301 },
        { id: 3, label: 'Finale', color: '#34D399', timestamp: 690 },
    ],
};
const sampleRating = 4;
const sampleLiked = true;
const sampleJizzCount = 7;

const config = computed<SceneCardConfig>({
    get: () => settingsStore.draft?.scene_card_config ?? settingsStore.sceneCardConfig,
    set: (val) => {
        if (settingsStore.draft) {
            settingsStore.draft.scene_card_config = val;
        }
    },
});

const contentFields = [
    { value: 'file_size', label: 'File Size', fullOnly: false, hasMode: false },
    { value: 'added_at', label: 'Date Added', fullOnly: false, hasMode: false },
    { value: 'views', label: 'Views', fullOnly: false, hasMode: false },
    { value: 'resolution', label: 'Resolution', fullOnly: false, hasMode: false },
    { value: 'frame_rate', label: 'Frame Rate', fullOnly: false, hasMode: false },
    { value: 'jizz_count', label: 'Jizz Count', fullOnly: false, hasMode: false },
    { value: 'rating', label: 'Rating', fullOnly: false, hasMode: false },
    { value: 'description', label: 'Description', fullOnly: true, hasMode: false },
    { value: 'tags', label: 'Tags & Markers & Actors', fullOnly: false, hasMode: true },
    { value: 'actors', label: 'Actors', fullOnly: false, hasMode: true },
    { value: 'studio', label: 'Studio', fullOnly: true, hasMode: false },
];

// Fields available in split rows (exclude fullOnly items like description, studio)
const splitFields = computed(() => contentFields.filter((f) => !f.fullOnly));

const zoneNames = ['top_left', 'top_right', 'bottom_left', 'bottom_right'] as const;
const zoneLabels: Record<string, string> = {
    top_left: 'Top Left',
    top_right: 'Top Right',
    bottom_left: 'Bottom Left',
    bottom_right: 'Bottom Right',
};

// Selected zone for editing
const selectedZone = ref<(typeof zoneNames)[number] | null>(null);

function updateZone(name: (typeof zoneNames)[number], zone: BadgeZone) {
    const newConfig = JSON.parse(JSON.stringify(config.value)) as SceneCardConfig;
    newConfig.badges[name] = zone;
    config.value = newConfig;
}

// Content rows
function addContentRow(type: 'full' | 'split') {
    const newConfig = JSON.parse(JSON.stringify(config.value)) as SceneCardConfig;
    if (type === 'full') {
        newConfig.content_rows.push({ type: 'full', field: 'file_size' });
    } else {
        newConfig.content_rows.push({ type: 'split', left: 'file_size', right: 'added_at' });
    }
    config.value = newConfig;
}

function removeContentRow(idx: number) {
    const newConfig = JSON.parse(JSON.stringify(config.value)) as SceneCardConfig;
    newConfig.content_rows.splice(idx, 1);
    config.value = newConfig;
}

function moveContentRow(idx: number, dir: -1 | 1) {
    const newConfig = JSON.parse(JSON.stringify(config.value)) as SceneCardConfig;
    const newIdx = idx + dir;
    const rows = newConfig.content_rows;
    if (newIdx < 0 || newIdx >= rows.length) return;
    const moved = rows[idx];
    const other = rows[newIdx];
    if (moved === undefined || other === undefined) return;
    rows[idx] = other;
    rows[newIdx] = moved;
    config.value = newConfig;
}

function updateContentRow(idx: number, row: ContentRow) {
    const newConfig = JSON.parse(JSON.stringify(config.value)) as SceneCardConfig;
    newConfig.content_rows[idx] = row;
    config.value = newConfig;
}
</script>

<template>
    <div class="space-y-5">
        <!-- Live Card Preview -->
        <div>
            <h4 class="mb-3 text-xs font-semibold text-white">Preview</h4>
            <div class="border-border bg-void/30 flex justify-center rounded-lg border p-6">
                <div class="w-[280px]">
                    <SceneCard
                        :scene="sampleScene"
                        :rating="sampleRating"
                        :liked="sampleLiked"
                        :jizz-count="sampleJizzCount"
                        :completed="true"
                        :config-override="config"
                        fluid
                        preview
                    />
                </div>
            </div>
        </div>

        <!-- Badge Zones -->
        <div>
            <h4 class="mb-3 text-xs font-semibold text-white">Thumbnail Badges</h4>
            <p class="text-dim mb-3 text-[11px]">
                Configure which badges appear on each corner of the thumbnail overlay. Set columns
                to lay a corner out as a grid of several rows and columns.
            </p>

            <div class="grid grid-cols-2 gap-3">
                <SettingsAppCardTemplateBadgeZoneEditor
                    v-for="zoneName in zoneNames"
                    :key="zoneName"
                    :zone="config.badges[zoneName]"
                    :label="zoneLabels[zoneName] ?? zoneName"
                    :open="selectedZone === zoneName"
                    @update:zone="(zone) => updateZone(zoneName, zone)"
                    @toggle="selectedZone = selectedZone === zoneName ? null : zoneName"
                />
            </div>
        </div>

        <!-- Content Rows -->
        <div>
            <h4 class="mb-3 text-xs font-semibold text-white">Content Rows</h4>
            <p class="text-dim mb-3 text-[11px]">
                Configure what information appears below the title. Rows are displayed in order.
            </p>

            <div class="space-y-2">
                <div
                    v-for="(row, idx) in config.content_rows"
                    :key="idx"
                    class="border-border bg-void/30 flex items-center gap-3 rounded-lg border p-3"
                >
                    <!-- Row type indicator -->
                    <span
                        class="shrink-0 rounded px-1.5 py-0.5 text-[9px] font-medium uppercase"
                        :class="
                            row.type === 'full'
                                ? 'bg-lava/20 text-lava'
                                : 'bg-blue-500/20 text-blue-400'
                        "
                    >
                        {{ row.type }}
                    </span>

                    <!-- Row config -->
                    <div v-if="row.type === 'full'" class="flex flex-1 items-center gap-2">
                        <select
                            :value="row.field"
                            class="border-border bg-void/80 rounded border px-2 py-1 text-[10px]
                                text-white"
                            @change="
                                (e) =>
                                    updateContentRow(idx, {
                                        ...row,
                                        field: (e.target as HTMLSelectElement).value,
                                    })
                            "
                        >
                            <option v-for="f in contentFields" :key="f.value" :value="f.value">
                                {{ f.label }}
                            </option>
                        </select>

                        <!-- Mode selector for tags/actors -->
                        <select
                            v-if="row.field === 'tags' || row.field === 'actors'"
                            :value="row.mode || 'long'"
                            class="border-border bg-void/80 rounded border px-2 py-1 text-[10px]
                                text-white"
                            @change="
                                (e) =>
                                    updateContentRow(idx, {
                                        ...row,
                                        mode: (e.target as HTMLSelectElement).value as
                                            | 'short'
                                            | 'long',
                                    })
                            "
                        >
                            <option value="short">Short (icon + popover)</option>
                            <option value="long">Long (inline)</option>
                        </select>
                    </div>

                    <div v-else class="flex flex-1 flex-wrap items-center gap-2">
                        <select
                            :value="row.left"
                            class="border-border bg-void/80 rounded border px-2 py-1 text-[10px]
                                text-white"
                            @change="
                                (e) =>
                                    updateContentRow(idx, {
                                        ...row,
                                        left: (e.target as HTMLSelectElement).value,
                                        left_mode:
                                            (e.target as HTMLSelectElement).value === 'tags' ||
                                            (e.target as HTMLSelectElement).value === 'actors'
                                                ? 'short'
                                                : undefined,
                                    })
                            "
                        >
                            <option v-for="f in splitFields" :key="f.value" :value="f.value">
                                {{ f.label }}
                            </option>
                        </select>
                        <select
                            v-if="row.left === 'tags' || row.left === 'actors'"
                            :value="row.left_mode || 'short'"
                            class="border-border bg-void/80 rounded border px-2 py-1 text-[10px]
                                text-white"
                            @change="
                                (e) =>
                                    updateContentRow(idx, {
                                        ...row,
                                        left_mode: (e.target as HTMLSelectElement).value as
                                            | 'short'
                                            | 'long',
                                    })
                            "
                        >
                            <option value="short">Short</option>
                            <option value="long">Long</option>
                        </select>
                        <span class="text-dim text-[10px]">|</span>
                        <select
                            :value="row.right"
                            class="border-border bg-void/80 rounded border px-2 py-1 text-[10px]
                                text-white"
                            @change="
                                (e) =>
                                    updateContentRow(idx, {
                                        ...row,
                                        right: (e.target as HTMLSelectElement).value,
                                        right_mode:
                                            (e.target as HTMLSelectElement).value === 'tags' ||
                                            (e.target as HTMLSelectElement).value === 'actors'
                                                ? 'short'
                                                : undefined,
                                    })
                            "
                        >
                            <option v-for="f in splitFields" :key="f.value" :value="f.value">
                                {{ f.label }}
                            </option>
                        </select>
                        <select
                            v-if="row.right === 'tags' || row.right === 'actors'"
                            :value="row.right_mode || 'short'"
                            class="border-border bg-void/80 rounded border px-2 py-1 text-[10px]
                                text-white"
                            @change="
                                (e) =>
                                    updateContentRow(idx, {
                                        ...row,
                                        right_mode: (e.target as HTMLSelectElement).value as
                                            | 'short'
                                            | 'long',
                                    })
                            "
                        >
                            <option value="short">Short</option>
                            <option value="long">Long</option>
                        </select>
                    </div>

                    <!-- Move/Remove buttons -->
                    <div class="flex shrink-0 gap-1">
                        <button
                            :disabled="idx === 0"
                            class="text-dim hover:text-white disabled:opacity-30"
                            @click="moveContentRow(idx, -1)"
                        >
                            <Icon name="heroicons:chevron-up" size="14" />
                        </button>
                        <button
                            :disabled="idx === config.content_rows.length - 1"
                            class="text-dim hover:text-white disabled:opacity-30"
                            @click="moveContentRow(idx, 1)"
                        >
                            <Icon name="heroicons:chevron-down" size="14" />
                        </button>
                        <button class="text-dim hover:text-lava" @click="removeContentRow(idx)">
                            <Icon name="heroicons:x-mark" size="14" />
                        </button>
                    </div>
                </div>
            </div>

            <!-- Add row buttons -->
            <div class="mt-3 flex gap-2">
                <button
                    class="border-border hover:border-lava/40 hover:bg-lava/10 rounded-lg border
                        px-3 py-1.5 text-[10px] font-medium text-white transition-all"
                    @click="addContentRow('full')"
                >
                    + Full-width Row
                </button>
                <button
                    class="border-border hover:border-lava/40 hover:bg-lava/10 rounded-lg border
                        px-3 py-1.5 text-[10px] font-medium text-white transition-all"
                    @click="addContentRow('split')"
                >
                    + Split Row
                </button>
            </div>
        </div>
    </div>
</template>
