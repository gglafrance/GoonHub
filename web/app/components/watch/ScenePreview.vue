<script setup lang="ts">
import type { Video } from '~/types/video';
import type { PornDBScene } from '~/types/porndb';

const props = defineProps<{
    video: Video | null;
    scene: PornDBScene;
}>();

const emit = defineEmits<{
    back: [];
    apply: [
        fields: {
            title: boolean;
            description: boolean;
            studio: boolean;
            thumbnail: boolean;
            performers: boolean;
            tags: boolean;
        },
    ];
    close: [];
}>();

const selectedFields = ref({
    title: false,
    description: false,
    studio: false,
    thumbnail: false,
    performers: false,
    tags: false,
});

function formatDuration(seconds: number): string {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

onMounted(() => {
    selectedFields.value = {
        title: !props.video?.title,
        description: !props.video?.description,
        studio: !props.video?.studio && !!props.scene.site?.name,
        thumbnail: !props.video?.thumbnail_path && !!(props.scene.image || props.scene.poster),
        performers: (props.scene.performers?.length ?? 0) > 0,
        tags: (props.scene.tags?.length ?? 0) > 0,
    };
});
</script>

<template>
    <div class="space-y-4">
        <!-- Back button -->
        <button
            @click="emit('back')"
            class="text-dim flex items-center gap-1 text-xs transition-colors hover:text-white"
        >
            <Icon name="heroicons:arrow-left" size="14" />
            Back to search
        </button>

        <!-- Scene header -->
        <div class="border-border bg-surface flex gap-4 rounded-lg border p-4">
            <div class="bg-void h-28 w-44 shrink-0 overflow-hidden rounded-lg">
                <img
                    v-if="scene.image || scene.poster"
                    :src="scene.image || scene.poster"
                    :alt="scene.title"
                    class="h-full w-full object-cover"
                />
                <div v-else class="text-dim flex h-full w-full items-center justify-center">
                    <Icon name="heroicons:film" size="28" />
                </div>
            </div>
            <div class="py-1">
                <p class="text-sm font-medium text-white">{{ scene.title }}</p>
                <p v-if="scene.site?.name" class="text-dim mt-0.5 text-xs">{{ scene.site.name }}</p>
                <div class="text-dim mt-2 flex flex-wrap gap-3 text-[11px]">
                    <span v-if="scene.date" class="flex items-center gap-1">
                        <Icon name="heroicons:calendar" size="12" />
                        {{ scene.date }}
                    </span>
                    <span v-if="scene.duration" class="flex items-center gap-1">
                        <Icon name="heroicons:clock" size="12" />
                        {{ formatDuration(scene.duration) }}
                    </span>
                </div>
            </div>
        </div>

        <!-- Fields comparison -->
        <div class="space-y-3">
            <h4 class="text-dim text-[11px] font-medium tracking-wider uppercase">
                Select fields to apply
            </h4>

            <div class="grid grid-cols-1 gap-2 lg:grid-cols-2">
                <!-- Title -->
                <label
                    class="border-border bg-surface flex items-start gap-3 rounded-lg border p-3"
                >
                    <input
                        v-model="selectedFields.title"
                        type="checkbox"
                        class="accent-lava mt-0.5 h-4 w-4 shrink-0 rounded"
                    />
                    <div class="min-w-0 flex-1">
                        <p class="text-xs font-medium text-white">Title</p>
                        <div class="mt-1.5 space-y-1 text-[11px]">
                            <div>
                                <span class="text-dim">Current:</span>
                                <span class="ml-1 text-white/70">{{
                                    video?.title || '(empty)'
                                }}</span>
                            </div>
                            <div>
                                <span class="text-dim">New:</span>
                                <span class="text-lava ml-1">{{ scene.title }}</span>
                            </div>
                        </div>
                    </div>
                </label>

                <!-- Studio -->
                <label
                    v-if="scene.site?.name"
                    class="border-border bg-surface flex items-start gap-3 rounded-lg border p-3"
                >
                    <input
                        v-model="selectedFields.studio"
                        type="checkbox"
                        class="accent-lava mt-0.5 h-4 w-4 shrink-0 rounded"
                    />
                    <div class="min-w-0 flex-1">
                        <p class="text-xs font-medium text-white">Studio</p>
                        <div class="mt-1.5 space-y-1 text-[11px]">
                            <div>
                                <span class="text-dim">Current:</span>
                                <span class="ml-1 text-white/70">{{
                                    video?.studio || '(empty)'
                                }}</span>
                            </div>
                            <div>
                                <span class="text-dim">New:</span>
                                <span class="text-lava ml-1">{{ scene.site.name }}</span>
                            </div>
                        </div>
                    </div>
                </label>

                <!-- Thumbnail -->
                <label
                    v-if="scene.image || scene.poster"
                    class="border-border bg-surface flex items-start gap-3 rounded-lg border p-3
                        lg:col-span-2"
                >
                    <input
                        v-model="selectedFields.thumbnail"
                        type="checkbox"
                        class="accent-lava mt-0.5 h-4 w-4 shrink-0 rounded"
                    />
                    <div class="min-w-0 flex-1">
                        <p class="text-xs font-medium text-white">Thumbnail</p>
                        <div class="mt-1.5 flex items-center gap-3">
                            <div class="bg-void h-16 w-28 shrink-0 overflow-hidden rounded">
                                <img
                                    :src="(scene.image || scene.poster)!"
                                    :alt="scene.title"
                                    class="h-full w-full object-cover"
                                />
                            </div>
                            <p class="text-dim text-[11px]">
                                Import scene thumbnail as video thumbnail
                            </p>
                        </div>
                    </div>
                </label>

                <!-- Description (full width) -->
                <label
                    v-if="scene.description"
                    class="border-border bg-surface flex items-start gap-3 rounded-lg border p-3
                        lg:col-span-2"
                >
                    <input
                        v-model="selectedFields.description"
                        type="checkbox"
                        class="accent-lava mt-0.5 h-4 w-4 shrink-0 rounded"
                    />
                    <div class="min-w-0 flex-1">
                        <p class="text-xs font-medium text-white">Description</p>
                        <div class="mt-1.5 grid grid-cols-2 gap-4 text-[11px]">
                            <div>
                                <span class="text-dim">Current:</span>
                                <p class="mt-0.5 line-clamp-3 text-white/70">
                                    {{ video?.description || '(empty)' }}
                                </p>
                            </div>
                            <div>
                                <span class="text-dim">New:</span>
                                <p class="text-lava mt-0.5 line-clamp-3">{{ scene.description }}</p>
                            </div>
                        </div>
                    </div>
                </label>

                <!-- Performers (full width) -->
                <label
                    v-if="scene.performers?.length"
                    class="border-border bg-surface flex items-start gap-3 rounded-lg border p-3
                        lg:col-span-2"
                >
                    <input
                        v-model="selectedFields.performers"
                        type="checkbox"
                        class="accent-lava mt-0.5 h-4 w-4 shrink-0 rounded"
                    />
                    <div class="min-w-0 flex-1">
                        <p class="text-xs font-medium text-white">
                            Performers ({{ scene.performers.length }})
                        </p>
                        <div class="mt-2 flex flex-wrap gap-2">
                            <div
                                v-for="performer in scene.performers"
                                :key="performer.id"
                                class="border-border flex items-center gap-1.5 rounded-full border
                                    px-2 py-0.5 text-[11px] text-white"
                            >
                                <img
                                    v-if="performer.image"
                                    :src="performer.image"
                                    class="h-4 w-4 rounded-full object-cover"
                                />
                                {{ performer.name }}
                            </div>
                        </div>
                        <p class="text-dim mt-1.5 text-[10px]">
                            You'll be able to match each performer to existing actors or create new
                            ones.
                        </p>
                    </div>
                </label>

                <!-- Tags (full width) -->
                <label
                    v-if="scene.tags?.length"
                    class="border-border bg-surface flex items-start gap-3 rounded-lg border p-3
                        lg:col-span-2"
                >
                    <input
                        v-model="selectedFields.tags"
                        type="checkbox"
                        class="accent-lava mt-0.5 h-4 w-4 shrink-0 rounded"
                    />
                    <div class="min-w-0 flex-1">
                        <p class="text-dim text-xs font-medium">Tags ({{ scene.tags.length }})</p>
                        <div class="mt-2 flex flex-wrap gap-1">
                            <span
                                v-for="tag in scene.tags.slice(0, 15)"
                                :key="tag.id"
                                class="text-dim rounded bg-white/5 px-1.5 py-0.5 text-[10px]"
                            >
                                {{ tag.name }}
                            </span>
                            <span v-if="scene.tags.length > 15" class="text-dim text-[10px]">
                                +{{ scene.tags.length - 15 }} more
                            </span>
                        </div>
                    </div>
                </label>
            </div>
        </div>

        <!-- Actions -->
        <div class="flex justify-end gap-2 pt-2">
            <button
                @click="emit('close')"
                class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors hover:text-white"
            >
                Cancel
            </button>
            <button
                @click="emit('apply', { ...selectedFields })"
                :disabled="!Object.values(selectedFields).some(Boolean)"
                class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs font-semibold
                    text-white transition-all disabled:cursor-not-allowed disabled:opacity-40"
            >
                Apply Selected
            </button>
        </div>
    </div>
</template>
