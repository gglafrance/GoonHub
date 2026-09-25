<script setup lang="ts">
import { formatShortClock } from '~/composables/useShortEditor';

const props = defineProps<{
    sourceSceneId: number;
    sourceStart: number;
}>();

const { fetchScene } = useApiScenes();

const sourceTitle = ref('');
const missing = ref(false);

const load = async () => {
    missing.value = false;
    try {
        const scene = await fetchScene(props.sourceSceneId);
        sourceTitle.value = scene.title;
    } catch {
        missing.value = true;
    }
};

onMounted(load);
watch(() => props.sourceSceneId, load);

const link = computed(() => `/watch/${props.sourceSceneId}?t=${Math.floor(props.sourceStart)}`);
</script>

<template>
    <div
        class="border-border bg-surface/60 flex items-center gap-2 rounded-lg border px-3 py-2
            text-xs"
    >
        <Icon name="heroicons:scissors" size="14" class="text-lava shrink-0" />
        <span class="text-dim">Created from</span>
        <span v-if="missing" class="text-dim italic">a scene that is no longer available</span>
        <NuxtLink
            v-else
            :to="link"
            class="hover:text-lava min-w-0 truncate font-medium text-white transition-colors"
        >
            {{ sourceTitle || `scene ${sourceSceneId}` }}
        </NuxtLink>
        <span class="text-dim shrink-0 font-mono">at {{ formatShortClock(sourceStart) }}</span>
    </div>
</template>
