<script setup lang="ts">
import type { ProfileSectionView } from '~/types/profile';

// The sections of a profile tab: each one a row of columns (its template, stacked on mobile),
// each column its blocks. Other viewers do not see empty blocks (a locked placeholder stays),
// nor sections left without blocks.
const props = defineProps<{ sections: ProfileSectionView[]; owner: boolean }>();

const rows = computed(() =>
    (props.sections ?? [])
        .map((s) => {
            const classes = sectionColumnClasses(s.template);
            const columns = s.columns.map((col, i) => ({
                key: `${s.id}-${i}`,
                cls: classes[i] ?? 'md:col-span-12',
                blocks: col.filter((b) => props.owner || b.locked || !b.empty),
            }));
            return { id: s.id, columns };
        })
        .filter((r) => props.owner || r.columns.some((c) => c.blocks.length > 0)),
);

const empty = computed(() =>
    rows.value.every((r) => r.columns.every((c) => c.blocks.length === 0)),
);
</script>

<template>
    <div class="space-y-4">
        <div v-for="row in rows" :key="row.id" class="grid grid-cols-1 gap-4 md:grid-cols-12">
            <div
                v-for="col in row.columns"
                :key="col.key"
                class="min-w-0 space-y-4"
                :class="[col.cls, col.blocks.length === 0 ? 'hidden md:block' : '']"
            >
                <ProfileBlockRenderer
                    v-for="block in col.blocks"
                    :key="block.id ?? block.type"
                    :block="block"
                    :owner="props.owner"
                />
            </div>
        </div>
        <div v-if="empty" class="glass-panel text-dim p-6 text-center text-xs">
            {{
                props.owner
                    ? 'This tab shows no block yet: Edit profile > Layout.'
                    : 'Nothing to show here.'
            }}
        </div>
    </div>
</template>
