<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// rediscover widget: favorites (liked, well rated, jizzed) not watched for a while.
interface RediscoverItem {
    last_watched: string | null;
    jizz_count: number;
    rating: number;
    liked: boolean;
    views: number;
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<{ days: number; items: Record<string, RediscoverItem> }>;
}>();

const items = computed(() => props.data.widget?.items ?? {});
const days = computed(() => props.data.widget?.days ?? 180);
const item = (sceneId: number) => items.value[String(sceneId)];
</script>

<template>
    <div>
        <HomepageWidgetsScenesToolbar>
            Favorites not watched for more than {{ days }} days
        </HomepageWidgetsScenesToolbar>
        <HomepageWidgetsScenesEmpty
            v-if="data.scenes.length === 0"
            icon="heroicons:heart"
            message="No forgotten favorites: like, rate or jizz on scenes to fill this section."
        />
        <div v-else class="scrollbar-hide -mx-4 flex gap-4 overflow-x-auto px-4 pb-2">
            <div v-for="scene in data.scenes" :key="scene.id" class="shrink-0">
                <SceneCard
                    :scene="scene"
                    :rating="data.ratings?.[scene.id]"
                    :liked="item(scene.id)?.liked"
                    :jizz-count="item(scene.id)?.jizz_count"
                />
                <div class="text-dim mt-1.5 flex items-center gap-2.5 px-0.5 text-[10px]">
                    <span class="flex items-center gap-1">
                        <Icon name="heroicons:clock" size="11" />
                        <template v-if="item(scene.id)?.last_watched">
                            Seen <NuxtTime :datetime="item(scene.id)!.last_watched!" relative />
                        </template>
                        <template v-else>Never watched</template>
                    </span>
                    <span
                        v-if="item(scene.id)?.jizz_count"
                        class="text-lava flex items-center gap-0.5"
                    >
                        <Icon name="heroicons:fire" size="11" />
                        {{ item(scene.id)!.jizz_count }}
                    </span>
                    <span
                        v-if="item(scene.id)?.rating"
                        class="flex items-center gap-0.5 text-amber-400"
                    >
                        <Icon name="heroicons:star-solid" size="11" />
                        {{ item(scene.id)!.rating }}
                    </span>
                    <Icon
                        v-if="item(scene.id)?.liked"
                        name="heroicons:heart-solid"
                        size="11"
                        class="text-pink-400"
                    />
                </div>
            </div>
        </div>
    </div>
</template>

<style scoped>
.scrollbar-hide {
    -ms-overflow-style: none;
    scrollbar-width: none;
}
.scrollbar-hide::-webkit-scrollbar {
    display: none;
}
</style>
