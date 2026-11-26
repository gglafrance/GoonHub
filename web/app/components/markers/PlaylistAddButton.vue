<script setup lang="ts">
import type { MarkerPlaylistFilter } from '~/types/playlist';

// "Add to playlist" for markers: one marker or a few (markerIds), or a whole listing (filter).
// Opens the playlist picker on the marker playlists (and creates one inline). Hidden without the
// playlist permissions. Inside a link (marker cards), the click does not follow it.
const props = withDefaults(
    defineProps<{
        markerIds?: number[];
        filter?: MarkerPlaylistFilter | null;
        // Markers listed by the filter (shown in the picker)
        count?: number;
        // icon: over a marker card; row: in a marker row (watch page Markers tab); button
        variant?: 'icon' | 'row' | 'button';
        label?: string;
    }>(),
    {
        markerIds: undefined,
        filter: null,
        count: 0,
        variant: 'button',
        label: 'Add to playlist',
    },
);

const perms = useMyPermissions();
const allowed = computed(() => perms.has('playlists:edit') || perms.has('playlists:create'));
const showModal = ref(false);

const open = (event: MouseEvent) => {
    event.preventDefault();
    event.stopPropagation();
    showModal.value = true;
};
</script>

<template>
    <template v-if="allowed">
        <button
            v-if="variant === 'icon'"
            type="button"
            class="hover:bg-lava/80 flex h-6 w-6 items-center justify-center rounded bg-black/70
                text-white/90 backdrop-blur-sm transition-colors"
            :title="label"
            :aria-label="label"
            @click="open"
        >
            <Icon name="heroicons:queue-list" size="13" />
        </button>
        <button
            v-else-if="variant === 'row'"
            type="button"
            class="text-dim hover:bg-surface-hover flex shrink-0 items-center justify-center rounded
                p-1.5 opacity-0 transition-all group-hover:opacity-100 hover:text-white
                focus-visible:opacity-100"
            :title="label"
            :aria-label="label"
            @click="open"
        >
            <Icon name="heroicons:queue-list" size="12" />
        </button>
        <button
            v-else
            type="button"
            class="border-border bg-panel text-dim hover:border-lava/30 flex items-center gap-1.5
                rounded-lg border px-2.5 py-1.5 text-[11px] font-medium transition-all
                hover:text-white"
            :title="label"
            @click="open"
        >
            <Icon name="heroicons:queue-list" size="13" />
            {{ label }}
        </button>

        <!-- Mounted on demand: marker grids hold one button per card -->
        <PlaylistCreateModal
            v-if="showModal"
            :visible="showModal"
            :prefill-marker-ids="props.markerIds"
            :marker-filter="props.markerIds?.length ? null : props.filter"
            :marker-filter-count="props.count"
            kind="markers"
            @close="showModal = false"
            @created="showModal = false"
        />
    </template>
</template>
