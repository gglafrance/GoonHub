<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// folder_latest widget: latest additions of a storage path and/or folder.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<{
        storage_path: { id: number; name: string } | null;
        folder: string;
    }>;
}>();

const source = computed(() => {
    const parts: string[] = [];
    if (props.data.widget?.storage_path) parts.push(props.data.widget.storage_path.name);
    if (props.data.widget?.folder) parts.push(props.data.widget.folder);
    return parts.join(' / ');
});
</script>

<template>
    <div>
        <HomepageWidgetsScenesToolbar>
            <Icon name="heroicons:folder-open" size="12" class="mr-1 inline" />
            <span class="font-mono text-white">{{ source || 'No folder selected' }}</span>
            <template v-if="data.total"> · {{ data.total.toLocaleString() }} scenes</template>
        </HomepageWidgetsScenesToolbar>
        <HomepageWidgetsScenesEmpty
            v-if="data.scenes.length === 0"
            icon="heroicons:folder"
            :message="
                source
                    ? 'No scene in this folder yet.'
                    : 'Edit the section to choose a storage path or a folder.'
            "
        />
        <HomepageSectionGrid
            v-else
            :scenes="data.scenes"
            :watch-progress="data.watch_progress"
            :ratings="data.ratings"
        />
    </div>
</template>
