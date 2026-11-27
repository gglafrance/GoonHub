<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// performer_spotlight widget: your top performers with their stats.
interface PerformerItem {
    id: number;
    uuid: string;
    name: string;
    image_url: string;
    scene_count: number;
    views: number;
    jizz: number;
    liked: number;
    last_watched?: string;
}

interface PerformerPayload {
    order: 'views' | 'jizz' | 'scenes';
    items: PerformerItem[];
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<PerformerPayload>;
}>();

defineEmits<{ refresh: [] }>();

const items = computed(() => props.data.widget?.items ?? []);
const number = new Intl.NumberFormat();

const initials = (name: string) =>
    name
        .split(/\s+/)
        .filter(Boolean)
        .slice(0, 2)
        .map((part) => part[0]!.toUpperCase())
        .join('');

// Images that failed to load fall back to initials
const brokenImages = ref<Set<number>>(new Set());
const onImageError = (id: number) => {
    brokenImages.value = new Set([...brokenImages.value, id]);
};
</script>

<template>
    <div
        v-if="items.length === 0"
        class="border-border text-dim flex h-32 items-center justify-center rounded-lg border
            border-dashed text-xs"
    >
        No performer stats yet: watch some scenes with tagged performers.
    </div>
    <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-8">
        <NuxtLink
            v-for="(item, index) in items"
            :key="item.id"
            :to="`/actors/${item.uuid}`"
            class="border-border hover:border-lava/40 group relative flex flex-col items-center
                rounded-lg border bg-white/2 p-3 text-center transition-all hover:bg-white/4"
        >
            <span
                class="text-dim absolute top-1.5 left-2 font-mono text-[10px]"
                :class="{ 'text-lava': index === 0 }"
                >#{{ index + 1 }}</span
            >
            <div
                class="border-border mb-2 flex h-14 w-14 items-center justify-center overflow-hidden
                    rounded-full border bg-white/5"
            >
                <img
                    v-if="item.image_url && !brokenImages.has(item.id)"
                    :src="item.image_url"
                    :alt="item.name"
                    loading="lazy"
                    class="h-full w-full object-cover"
                    @error="onImageError(item.id)"
                />
                <span v-else class="text-sm font-semibold text-white/70">{{
                    initials(item.name)
                }}</span>
            </div>
            <div class="w-full truncate text-xs font-medium text-white" :title="item.name">
                {{ item.name }}
            </div>
            <div class="text-dim mt-1 flex flex-wrap justify-center gap-x-2 font-mono text-[10px]">
                <span :title="`${item.views} views`">
                    <Icon name="heroicons:eye" size="10" /> {{ number.format(item.views) }}
                </span>
                <span v-if="item.jizz" :title="`${item.jizz} jizz`">
                    <Icon name="heroicons:fire" size="10" /> {{ number.format(item.jizz) }}
                </span>
                <span v-if="item.liked" :title="`${item.liked} liked scenes`">
                    <Icon name="heroicons:heart" size="10" /> {{ item.liked }}
                </span>
            </div>
            <div class="text-dim mt-0.5 text-[10px]">
                {{ number.format(item.scene_count) }} scene{{ item.scene_count === 1 ? '' : 's' }}
            </div>
            <div v-if="item.last_watched" class="mt-0.5 text-[9px] text-white/30">
                <NuxtTime :datetime="item.last_watched" relative />
            </div>
        </NuxtLink>
    </div>
</template>
