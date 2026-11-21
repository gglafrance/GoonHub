<script setup lang="ts">
import type { HealthCategory, HealthSceneRow } from '~/types/libraryHealth';

const props = defineProps<{
    category: HealthCategory;
}>();

const emit = defineEmits<{
    close: [];
}>();

const PAGE_SIZE = 25;

const { listScenes } = useApiLibraryHealth();
const { formatSize, formatDuration } = useFormatter();

const loading = ref(false);
const error = ref('');
const rows = ref<HealthSceneRow[]>([]);
const page = ref(1);
const total = ref(0);
const totalPages = ref(0);
const panelRef = ref<HTMLElement | null>(null);

const info = computed(() => healthCategoryInfo(props.category));

const load = async () => {
    loading.value = true;
    error.value = '';
    try {
        const data = await listScenes(props.category, page.value, PAGE_SIZE);
        rows.value = data.data;
        total.value = data.pagination.total_items;
        totalPages.value = data.pagination.total_pages;
        // A refresh can shrink the list below the current page
        if (page.value > 1 && page.value > totalPages.value) {
            page.value = Math.max(1, totalPages.value);
            await load();
        }
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load scenes';
    } finally {
        loading.value = false;
    }
};

const goTo = (p: number) => {
    page.value = p;
    load();
};

// The end of a path says more than its start (the storage root)
const tailOf = (path: string, max = 60): string =>
    path.length <= max ? path : `...${path.slice(-(max - 3))}`;

const codecs = (row: HealthSceneRow): string => {
    const container = row.stored_path.split('.').pop()?.toLowerCase() ?? '';
    return [container, row.video_codec, row.audio_codec].filter(Boolean).join(' / ');
};

const rowInfo = (row: HealthSceneRow): string => {
    if (row.detail) return row.detail;
    switch (props.category) {
        case 'unplayable':
            return codecs(row);
        case 'size_zero':
        case 'duration_zero':
        case 'corrupted':
            return [
                row.size ? formatSize(row.size) : '0 B',
                row.duration ? formatDuration(row.duration) : 'no duration',
            ].join(' / ');
        default:
            return [row.size ? formatSize(row.size) : '', codecs(row)].filter(Boolean).join(' / ');
    }
};

onMounted(async () => {
    await load();
    panelRef.value?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
});

defineExpose({ reload: load });
</script>

<template>
    <div ref="panelRef" class="glass-panel p-5">
        <div class="mb-3 flex items-start justify-between gap-4">
            <div>
                <h3 class="text-sm font-semibold text-white">{{ info?.label }}</h3>
                <p class="text-dim mt-0.5 text-[11px]">
                    {{ total.toLocaleString() }} scenes &middot; {{ info?.description }}
                </p>
            </div>
            <button
                class="text-dim rounded p-1 transition-colors hover:bg-white/10 hover:text-white"
                title="Close"
                @click="emit('close')"
            >
                <Icon name="heroicons:x-mark" size="16" />
            </button>
        </div>

        <div v-if="error" class="text-lava py-4 text-center text-xs">{{ error }}</div>
        <div v-else-if="loading && rows.length === 0" class="text-dim py-6 text-center text-xs">
            Loading...
        </div>
        <div v-else-if="rows.length === 0" class="text-dim py-6 text-center text-xs">No scenes</div>
        <div v-else class="overflow-x-auto" :class="{ 'opacity-60': loading }">
            <table class="w-full table-fixed text-left text-xs">
                <thead>
                    <tr class="text-dim border-border border-b text-[10px] uppercase">
                        <th class="w-[35%] pr-3 pb-2 font-medium">Scene</th>
                        <th class="w-[40%] pr-3 pb-2 font-medium">Path</th>
                        <th class="pb-2 font-medium">Info</th>
                    </tr>
                </thead>
                <tbody>
                    <tr
                        v-for="row in rows"
                        :key="row.id"
                        class="border-border/50 border-b last:border-0"
                    >
                        <td class="py-2 pr-3">
                            <NuxtLink
                                :to="`/watch/${row.id}`"
                                class="hover:text-lava block truncate text-white transition-colors"
                                :title="row.title"
                            >
                                {{ row.title || `Scene ${row.id}` }}
                            </NuxtLink>
                        </td>
                        <td class="py-2 pr-3">
                            <code
                                class="text-dim bg-void/50 block truncate rounded px-1.5 py-0.5
                                    text-[11px]"
                                :title="row.stored_path"
                            >
                                {{ tailOf(row.stored_path) }}
                            </code>
                        </td>
                        <td class="text-dim truncate py-2 text-[11px]" :title="rowInfo(row)">
                            {{ rowInfo(row) }}
                        </td>
                    </tr>
                </tbody>
            </table>
        </div>

        <div
            v-if="totalPages > 1"
            class="border-border mt-3 flex items-center justify-end gap-3 border-t pt-3"
        >
            <button
                :disabled="page <= 1 || loading"
                class="text-dim disabled:hover:text-dim text-[11px] transition-colors
                    hover:text-white disabled:opacity-30"
                @click="goTo(page - 1)"
            >
                Previous
            </button>
            <span class="text-dim text-[11px]">{{ page }} / {{ totalPages }}</span>
            <button
                :disabled="page >= totalPages || loading"
                class="text-dim disabled:hover:text-dim text-[11px] transition-colors
                    hover:text-white disabled:opacity-30"
                @click="goTo(page + 1)"
            >
                Next
            </button>
        </div>
    </div>
</template>
