<script setup lang="ts">
import type { BadgeZone } from '~/types/settings';

const props = defineProps<{
    zone: BadgeZone;
    label: string;
    open: boolean;
}>();

const emit = defineEmits<{
    'update:zone': [zone: BadgeZone];
    toggle: [];
}>();

const badgeFields = [
    { value: 'liked', label: 'Liked' },
    { value: 'rating', label: 'Rating' },
    { value: 'duration', label: 'Duration' },
    { value: 'resolution', label: 'Resolution' },
    { value: 'views', label: 'Views' },
    { value: 'jizz_count', label: 'Jizz Count' },
    { value: 'watched', label: 'Watched' },
    { value: 'file_size', label: 'File Size' },
    { value: 'added_at', label: 'Date Added' },
    { value: 'frame_rate', label: 'Frame Rate' },
    { value: 'tags', label: 'Tags & Markers & Actors' },
    { value: 'actors', label: 'Actors' },
];

// 0 is the original layout: one column (vertical) or one row (horizontal)
const columnOptions = [0, ...Array.from({ length: MAX_BADGE_ZONE_COLUMNS }, (_, i) => i + 1)];

const maxItems = MAX_BADGE_ZONE_ITEMS;
const columns = computed(() => zoneColumns(props.zone));
const cells = computed(() => zoneGridCells(props.zone));

function fieldLabel(value: string): string {
    return badgeFields.find((f) => f.value === value)?.label || value;
}

function update(patch: Partial<BadgeZone>) {
    const zone: BadgeZone = { ...props.zone, items: [...props.zone.items], ...patch };
    // Columns left out (not 0) for the original layout, as saved before the grid
    if (!zone.columns) delete zone.columns;
    emit('update:zone', zone);
}

function setColumns(value: number) {
    update({ columns: value || undefined });
}

function toggleDirection() {
    update({ direction: props.zone.direction === 'vertical' ? 'horizontal' : 'vertical' });
}

function addBadge(field: string) {
    if (!field || props.zone.items.includes(field)) return;
    if (props.zone.items.length >= maxItems) return;
    update({ items: [...props.zone.items, field] });
}

function removeBadge(idx: number) {
    const items = [...props.zone.items];
    items.splice(idx, 1);
    update({ items });
}

function moveBadge(idx: number, dir: -1 | 1) {
    const items = [...props.zone.items];
    const newIdx = idx + dir;
    const moved = items[idx];
    const other = items[newIdx];
    if (moved === undefined || other === undefined) return;
    items[idx] = other;
    items[newIdx] = moved;
    update({ items });
}

// Badge fields not used in the zone yet
const availableFields = computed(() =>
    props.zone.items.length >= maxItems
        ? []
        : badgeFields.filter((f) => !props.zone.items.includes(f.value)),
);

const directionLabel = computed(() => {
    if (columns.value === 0) return props.zone.direction;
    return props.zone.direction === 'vertical' ? 'fill columns first' : 'fill rows first';
});
</script>

<template>
    <div
        class="border-border rounded-lg border p-3 transition-all"
        :class="open ? 'border-lava/40 bg-lava/5' : 'hover:border-border-hover'"
    >
        <div class="mb-2 flex items-center justify-between">
            <span class="text-[11px] font-medium text-white">{{ label }}</span>
            <button
                class="text-dim hover:text-lava text-[10px] transition-colors"
                @click="emit('toggle')"
            >
                {{ open ? 'Close' : 'Edit' }}
            </button>
        </div>

        <!-- Badge items preview: the grid of the zone, or the list for the original layout -->
        <div
            v-if="columns > 0 && zone.items.length > 0"
            class="grid gap-1"
            :style="{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }"
        >
            <template v-for="(row, r) in cells" :key="r">
                <span
                    v-for="(cell, c) in row"
                    :key="`${r}-${c}`"
                    class="truncate rounded border px-1.5 py-0.5 text-[9px]"
                    :class="
                        cell === null
                            ? 'border-border/50 border-dashed text-transparent'
                            : 'border-border bg-void/50 text-white/70'
                    "
                >
                    {{ cell === null ? '-' : fieldLabel(zone.items[cell] ?? '') }}
                </span>
            </template>
        </div>
        <div v-else class="flex flex-wrap gap-1">
            <span
                v-for="item in zone.items"
                :key="item"
                class="border-border bg-void/50 rounded border px-1.5 py-0.5 text-[9px]
                    text-white/70"
            >
                {{ fieldLabel(item) }}
            </span>
            <span v-if="zone.items.length === 0" class="text-dim text-[9px]">Empty</span>
        </div>

        <div class="mt-2 flex flex-wrap items-center justify-between gap-2">
            <!-- Direction toggle (fill order of the grid) -->
            <button
                class="text-dim flex items-center gap-1 text-[10px] transition-colors
                    hover:text-white"
                @click="toggleDirection"
            >
                <Icon
                    :name="
                        zone.direction === 'vertical'
                            ? 'heroicons:arrows-up-down'
                            : 'heroicons:arrows-right-left'
                    "
                    size="12"
                />
                {{ directionLabel }}
            </button>

            <!-- Columns -->
            <div class="flex items-center gap-1.5">
                <span class="text-dim text-[10px]">Columns</span>
                <div class="border-border bg-void/50 flex rounded border p-0.5">
                    <button
                        v-for="value in columnOptions"
                        :key="value"
                        class="rounded px-1.5 py-0.5 text-[10px] transition-colors"
                        :class="
                            columns === value ? 'bg-lava/20 text-lava' : 'text-dim hover:text-white'
                        "
                        :title="
                            value === 0
                                ? 'One column (vertical) or one row (horizontal)'
                                : `Grid of ${value} column${value > 1 ? 's' : ''}`
                        "
                        :aria-pressed="columns === value"
                        @click="setColumns(value)"
                    >
                        {{ value === 0 ? 'Auto' : value }}
                    </button>
                </div>
            </div>
        </div>

        <!-- Expanded edit panel -->
        <div v-if="open" class="border-border mt-3 space-y-2 border-t pt-3">
            <!-- Current items with reorder/remove -->
            <div v-for="(item, idx) in zone.items" :key="item" class="flex items-center gap-2">
                <span class="text-[10px] text-white/80">{{ fieldLabel(item) }}</span>
                <div class="ml-auto flex gap-1">
                    <button
                        :disabled="idx === 0"
                        class="text-dim hover:text-white disabled:opacity-30"
                        @click="moveBadge(idx, -1)"
                    >
                        <Icon name="heroicons:chevron-up" size="12" />
                    </button>
                    <button
                        :disabled="idx === zone.items.length - 1"
                        class="text-dim hover:text-white disabled:opacity-30"
                        @click="moveBadge(idx, 1)"
                    >
                        <Icon name="heroicons:chevron-down" size="12" />
                    </button>
                    <button class="text-dim hover:text-lava" @click="removeBadge(idx)">
                        <Icon name="heroicons:x-mark" size="12" />
                    </button>
                </div>
            </div>

            <!-- Add badge dropdown -->
            <select
                v-if="availableFields.length > 0"
                class="border-border bg-void/80 w-full rounded border px-2 py-1 text-[10px]
                    text-white"
                @change="
                    (e) => {
                        addBadge((e.target as HTMLSelectElement).value);
                        (e.target as HTMLSelectElement).value = '';
                    }
                "
            >
                <option value="">Add badge...</option>
                <option v-for="f in availableFields" :key="f.value" :value="f.value">
                    {{ f.label }}
                </option>
            </select>
            <p class="text-dim text-[10px]">{{ zone.items.length }} / {{ maxItems }} badges</p>
        </div>
    </div>
</template>
