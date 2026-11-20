<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// roulette widget: "Surprise me" plays the drawn scene and draws the next one.
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<{ candidates: number }>;
}>();

const emit = defineEmits<{
    refresh: [];
}>();

const router = useRouter();
const { formatDuration } = useFormatter();
const { busy, request } = useSceneWidgetRefresh(
    () => props.data,
    () => emit('refresh'),
);

const pick = computed(() => props.data.scenes[0] ?? null);
const candidates = computed(() => props.data.widget?.candidates ?? props.data.total);

const thumbnailUrl = computed(() => {
    if (!pick.value?.thumbnail_path) return null;
    const v = pick.value.updated_at ? new Date(pick.value.updated_at).getTime() : '';
    return `/thumbnails/${pick.value.id}${v ? `?v=${v}` : ''}`;
});

const surprise = () => {
    if (!pick.value) return;
    const id = pick.value.id;
    // Draw the next scene for when the user comes back
    emit('refresh');
    router.push(`/watch/${id}`);
};
</script>

<template>
    <div
        class="border-border flex flex-col gap-4 rounded-xl border bg-linear-to-r from-white/3
            to-transparent p-4 sm:flex-row sm:items-center"
    >
        <button
            class="bg-lava hover:bg-lava-glow group flex shrink-0 items-center justify-center gap-3
                rounded-xl px-6 py-4 text-sm font-semibold text-white shadow-lg transition-all
                hover:scale-[1.02] active:scale-[0.98] disabled:opacity-50 sm:w-56"
            :disabled="!pick"
            @click="surprise"
        >
            <Icon
                name="heroicons:cube"
                size="22"
                class="transition-transform duration-500 group-hover:rotate-180"
            />
            Surprise me
        </button>

        <div v-if="pick" class="flex min-w-0 flex-1 items-center gap-3">
            <NuxtLink
                :to="`/watch/${pick.id}`"
                class="bg-void relative h-16 w-28 shrink-0 overflow-hidden rounded-md"
            >
                <img
                    v-if="thumbnailUrl"
                    :src="thumbnailUrl"
                    :alt="pick.title"
                    class="h-full w-full object-cover"
                />
            </NuxtLink>
            <div class="min-w-0">
                <div class="text-dim text-[10px] uppercase">Next pick</div>
                <NuxtLink
                    :to="`/watch/${pick.id}`"
                    class="hover:text-lava block truncate text-xs font-medium text-white"
                    :title="pick.title"
                >
                    {{ pick.title }}
                </NuxtLink>
                <div class="text-dim font-mono text-[10px]">
                    {{ formatDuration(pick.duration) }} · among
                    {{ candidates.toLocaleString() }} candidates
                </div>
            </div>
            <button
                class="text-dim hover:text-lava ml-auto shrink-0 p-2 transition-colors"
                title="Draw another one"
                :disabled="busy"
                @click="request"
            >
                <Icon name="heroicons:arrow-path" size="16" :class="{ 'animate-spin': busy }" />
            </button>
        </div>
        <p v-else class="text-dim text-xs">
            No scene matches the roulette criteria. Edit the section to widen them.
        </p>
    </div>
</template>
