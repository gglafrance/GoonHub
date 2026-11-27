<script setup lang="ts">
import type { SceneCopy } from '~/types/scene';

// Read-only list of the files the library scan recognised as copies of this scene (same
// content at another path). Renders nothing when the scene has no copies.
const props = defineProps<{
    sceneId: number;
}>();

const { fetchSceneCopies } = useApiScenes();
// Copies are file paths: files:view_paths (the API refuses the request otherwise)
const perms = useMyPermissions();
const { formatSize } = useFormatter();

const copies = ref<SceneCopy[]>([]);
const expanded = ref(false);

const load = async () => {
    const id = props.sceneId;
    if (!perms.has('files:view_paths')) return;
    try {
        const res = await fetchSceneCopies(id);
        if (id === props.sceneId) copies.value = res.copies ?? [];
    } catch {
        // Informational only: hide the list on error
        if (id === props.sceneId) copies.value = [];
    }
};

watch(
    () => props.sceneId,
    () => {
        expanded.value = false;
        copies.value = [];
        load();
    },
    { immediate: true },
);
</script>

<template>
    <div v-if="copies.length > 0" class="border-border border-b py-2.5">
        <button
            type="button"
            class="text-dim flex w-full items-center justify-between text-[11px] transition-colors
                hover:text-white"
            :aria-expanded="expanded"
            title="Files with the same content at other paths (not separate scenes)"
            @click="expanded = !expanded"
        >
            <span>Other copies ({{ copies.length }})</span>
            <Icon
                name="heroicons:chevron-down"
                size="12"
                class="transition-transform"
                :class="{ 'rotate-180': expanded }"
            />
        </button>
        <ul v-if="expanded" class="mt-1.5 space-y-1.5">
            <li v-for="copy in copies" :key="copy.id">
                <p class="text-dim/70 font-mono text-[10px] leading-relaxed break-all">
                    {{ copy.stored_path }}
                </p>
                <p class="text-dim/40 font-mono text-[10px]">
                    {{ formatSize(copy.size) }} · seen
                    <NuxtTime :datetime="copy.last_seen_at" relative />
                </p>
            </li>
        </ul>
    </div>
</template>
