<script setup lang="ts">
import type { TagDuplicateGroup } from '~/types/tag';

// Tags whose names only differ by case or spaces ("Cumshot" / "cumshot "), each group with a
// one-click merge (confirmed in the merge modal) into the tag with the most scenes.
defineProps<{
    groups: TagDuplicateGroup[];
}>();

const emit = defineEmits<{
    merge: [group: TagDuplicateGroup];
}>();
</script>

<template>
    <div v-if="groups.length > 0" class="glass-panel border-lava/20 border p-5">
        <div class="mb-4 flex items-center gap-2">
            <Icon name="heroicons:document-duplicate" size="16" class="text-lava" />
            <h3 class="text-sm font-semibold text-white">Possible Duplicates</h3>
            <span class="text-dim text-[11px]">
                {{ groups.length }} {{ groups.length === 1 ? 'group' : 'groups' }} of tags that
                differ only by case or spaces
            </span>
        </div>
        <div class="space-y-1.5">
            <div
                v-for="group in groups"
                :key="group.key"
                class="border-border flex items-center gap-3 rounded-lg border px-3 py-2"
            >
                <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
                    <span
                        v-for="tag in group.tags"
                        :key="tag.id"
                        class="flex items-center gap-1.5 rounded-full border px-2 py-0.5
                            text-[11px]"
                        :class="
                            tag.id === group.suggested_target_id
                                ? 'border-lava/40 bg-lava/5 text-white'
                                : 'border-border text-muted'
                        "
                        :title="
                            tag.id === group.suggested_target_id
                                ? 'Kept by default (most scenes)'
                                : undefined
                        "
                    >
                        <span
                            class="h-2 w-2 rounded-full"
                            :style="{ backgroundColor: tag.color }"
                        />
                        <span class="whitespace-pre">{{ tag.name }}</span>
                        <span class="text-dim">{{ tag.scene_count }}</span>
                    </span>
                </div>
                <button
                    class="border-border hover:border-lava/40 hover:text-lava text-muted shrink-0
                        rounded-lg border px-3 py-1 text-[11px] font-semibold transition-all"
                    @click="emit('merge', group)"
                >
                    Merge
                </button>
            </div>
        </div>
    </div>
</template>
