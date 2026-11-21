<script setup lang="ts">
import type { Scene } from '~/types/scene';

// Title, technical facts and the user's rating and like of the triaged scene.
const props = defineProps<{
    scene: Scene;
    rating: number;
    liked: boolean;
}>();

const emit = defineEmits<{
    rate: [rating: number];
    like: [];
}>();

const { formatDuration, formatSize } = useFormatter();

const resolution = computed(() => {
    const h = props.scene.height;
    if (!h) return '';
    if (h >= 2160) return '4K';
    if (h >= 1440) return '1440p';
    if (h >= 1080) return '1080p';
    if (h >= 720) return '720p';
    if (h >= 480) return '480p';
    return `${h}p`;
});

const facts = computed(() => {
    const out: { icon: string; label: string; title?: string }[] = [];
    if (props.scene.duration > 0)
        out.push({ icon: 'heroicons:clock', label: formatDuration(props.scene.duration) });
    if (resolution.value)
        out.push({
            icon: 'heroicons:tv',
            label: resolution.value,
            title:
                props.scene.width && props.scene.height
                    ? `${props.scene.width}x${props.scene.height}`
                    : undefined,
        });
    if (props.scene.size > 0)
        out.push({ icon: 'heroicons:circle-stack', label: formatSize(props.scene.size) });
    return out;
});

// Half ratings (set on the watch page) show as a full star
const starIcon = (i: number) =>
    props.rating >= i - 0.5 ? 'heroicons:star-solid' : 'heroicons:star';
</script>

<template>
    <div class="space-y-2">
        <div class="flex items-start justify-between gap-3">
            <NuxtLink
                :to="`/watch/${scene.id}`"
                target="_blank"
                class="hover:text-lava min-w-0 text-base leading-snug font-semibold text-white
                    transition-colors"
                :title="scene.original_filename"
            >
                {{ scene.title || scene.original_filename || `Scene ${scene.id}` }}
                <Icon
                    name="heroicons:arrow-top-right-on-square"
                    size="12"
                    class="text-dim ml-1 align-baseline"
                />
            </NuxtLink>
        </div>

        <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span
                v-for="fact in facts"
                :key="fact.icon"
                class="text-muted flex items-center gap-1 font-mono text-[11px]"
                :title="fact.title"
            >
                <Icon :name="fact.icon" size="12" class="text-dim" />
                {{ fact.label }}
            </span>
            <span class="text-dim flex items-center gap-1 text-[11px]">
                Added <UiFormattedDate :datetime="scene.created_at" />
            </span>
        </div>

        <div class="flex items-center gap-3 pt-1">
            <div class="flex items-center gap-0.5" role="group" aria-label="Rating">
                <button
                    v-for="i in 5"
                    :key="i"
                    class="rounded p-0.5 transition-transform hover:scale-110"
                    :title="`Rate ${i}/5 (${i})`"
                    tabindex="-1"
                    @click="emit('rate', rating === i ? 0 : i)"
                >
                    <Icon
                        :name="starIcon(i)"
                        size="18"
                        :class="rating >= i - 0.5 ? 'text-lava' : 'text-dim'"
                    />
                </button>
                <span class="text-dim ml-1 font-mono text-[10px]">{{
                    rating > 0 ? `${rating}/5` : 'Unrated'
                }}</span>
            </div>

            <button
                class="border-border flex items-center gap-1 rounded-md border px-2 py-1 text-[11px]
                    transition-all"
                :class="
                    liked
                        ? 'border-lava/40 bg-lava/10 text-lava'
                        : 'text-dim hover:border-lava/30 hover:text-white'
                "
                title="Like (L)"
                tabindex="-1"
                @click="emit('like')"
            >
                <Icon :name="liked ? 'heroicons:heart-solid' : 'heroicons:heart'" size="13" />
                {{ liked ? 'Liked' : 'Like' }}
            </button>
        </div>
    </div>
</template>
