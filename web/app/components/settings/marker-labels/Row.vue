<script setup lang="ts">
import type { MarkerLabelWithUsage } from '~/types/markerLabel';
import type { Tag } from '~/types/tag';

// One label of Settings > Marker Labels: color, name (rename), usage, description and default
// tags (expanded on demand), delete.
defineProps<{
    label: MarkerLabelWithUsage;
    selected: boolean;
}>();

const emit = defineEmits<{
    select: [on: boolean];
    color: [color: string];
    renamed: [name: string];
    tags: [tags: Tag[]];
    mergeInto: [target: { id: number; name: string }];
    delete: [];
}>();

const expanded = ref(false);
</script>

<template>
    <div
        class="border-border group rounded-lg border px-3 py-2 transition-colors hover:bg-white/2"
        :class="selected ? 'border-lava/30 bg-lava/3' : ''"
    >
        <div class="flex items-center gap-3">
            <input
                type="checkbox"
                :checked="selected"
                class="accent-lava h-3.5 w-3.5 rounded"
                :aria-label="`Select ${label.name}`"
                @change="emit('select', ($event.target as HTMLInputElement).checked)"
            />
            <SettingsTagsColorPicker :color="label.color" @select="(c) => emit('color', c)" />
            <SettingsMarkerLabelsNameEditor
                :label="label"
                @renamed="(name) => emit('renamed', name)"
                @merge-into="(target) => emit('mergeInto', target)"
            />
            <span
                class="text-dim text-[11px]"
                :title="`${plural(label.user_count, 'user')} use this label`"
                >{{ plural(label.marker_count, 'marker') }}</span
            >
            <button
                class="text-dim flex items-center gap-1 text-[11px] transition-colors
                    hover:text-white"
                :title="expanded ? 'Hide the default tags' : 'Default tags and description'"
                @click="expanded = !expanded"
            >
                <Icon name="heroicons:tag" size="12" />
                {{ label.default_tags.length }}
                <Icon
                    :name="expanded ? 'heroicons:chevron-up' : 'heroicons:chevron-down'"
                    size="12"
                />
            </button>
            <button
                class="text-dim flex items-center justify-center opacity-0 transition-all
                    group-hover:opacity-100 hover:text-red-400"
                title="Delete label"
                @click="emit('delete')"
            >
                <Icon name="heroicons:trash" size="14" />
            </button>
        </div>
        <div v-if="expanded" class="mt-3 space-y-3 pl-7">
            <SettingsMarkerLabelsDescriptionEditor :label="label" />
            <SettingsMarkerLabelsDefaultTags
                :label="label"
                @updated="(tags) => emit('tags', tags)"
            />
        </div>
    </div>
</template>
