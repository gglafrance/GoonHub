<script setup lang="ts">
import type { Scene } from '~/types/scene';
import type { PornDBScene } from '~/types/porndb';

const props = defineProps<{
    scene: Scene | null;
    porndbScene: PornDBScene;
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
            release_date: boolean;
            markers: boolean;
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
    release_date: false,
    markers: false,
});

function formatTime(seconds: number): string {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

onMounted(() => {
    selectedFields.value = {
        title: !props.scene?.title,
        description: !props.scene?.description,
        studio: !props.scene?.studio && !!props.porndbScene.site?.name,
        thumbnail:
            !props.scene?.thumbnail_path && !!(props.porndbScene.image || props.porndbScene.poster),
        performers: (props.porndbScene.performers?.length ?? 0) > 0,
        tags: (props.porndbScene.tags?.length ?? 0) > 0,
        release_date: !!props.porndbScene.date && !props.scene?.release_date,
        markers: (props.porndbScene.markers?.length ?? 0) > 0,
    };
});
</script>

<template>
    <div class="space-y-4">
        <!-- Back button -->
        <button
            class="text-dim flex items-center gap-1 text-xs transition-colors hover:text-white"
            @click="emit('back')"
        >
            <Icon name="heroicons:arrow-left" size="14" />
            Back to search
        </button>

        <!-- Scene header -->
        <div class="border-border bg-surface flex gap-4 rounded-lg border p-4">
            <div class="bg-void h-28 w-44 shrink-0 overflow-hidden rounded-lg">
                <img
                    v-if="porndbScene.image || porndbScene.poster"
                    :src="porndbScene.image || porndbScene.poster"
                    :alt="porndbScene.title"
                    class="h-full w-full object-cover"
                />
                <div v-else class="text-dim flex h-full w-full items-center justify-center">
                    <Icon name="heroicons:film" size="28" />
                </div>
            </div>
            <div class="py-1">
                <p class="text-sm font-medium text-white">{{ porndbScene.title }}</p>
                <p v-if="porndbScene.site?.name" class="text-dim mt-0.5 text-xs">
                    {{ porndbScene.site.name }}
                </p>
                <div class="text-dim mt-2 flex flex-wrap gap-3 text-[11px]">
                    <span v-if="porndbScene.date" class="flex items-center gap-1">
                        <Icon name="heroicons:calendar" size="12" />
                        {{ porndbScene.date }}
                    </span>
                    <span v-if="porndbScene.duration" class="flex items-center gap-1">
                        <Icon name="heroicons:clock" size="12" />
                        {{ formatTime(porndbScene.duration) }}
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
                                    scene?.title || '(empty)'
                                }}</span>
                            </div>
                            <div>
                                <span class="text-dim">New:</span>
                                <span class="text-lava ml-1">{{ porndbScene.title }}</span>
                            </div>
                        </div>
                    </div>
                </label>

                <!-- Studio -->
                <label
                    v-if="porndbScene.site?.name"
                    class="border-border bg-surface flex items-start gap-3 rounded-lg border p-3"
                >
                    <input
                        v-model="selectedFields.studio"
                        type="checkbox"
                        class="accent-lava mt-0.5 h-4 w-4 shrink-0 rounded"
                    />
                    <div class="min-w-0 flex-1">
                        <p class="text-xs font-medium text-white">Studio</p>
                        <div class="mt-1.5">
                            <div
                                class="border-border flex items-center gap-1.5 rounded-full border
                                    px-2 py-0.5 text-[11px] text-white"
                            >
                                {{ porndbScene.site.name }}
                            </div>
                        </div>
                        <p class="text-dim mt-1.5 text-[10px]">
                            You'll be able to match to an existing studio or create a new one.
                        </p>
                    </div>
                </label>

                <!-- Release Date -->
                <label
                    v-if="porndbScene.date"
                    class="border-border bg-surface flex items-start gap-3 rounded-lg border p-3"
                >
                    <input
                        v-model="selectedFields.release_date"
                        type="checkbox"
                        class="accent-lava mt-0.5 h-4 w-4 shrink-0 rounded"
                    />
                    <div class="min-w-0 flex-1">
                        <p class="text-xs font-medium text-white">Release Date</p>
                        <div class="mt-1.5 space-y-1 text-[11px]">
                            <div>
                                <span class="text-dim">Current:</span>
                                <span class="ml-1 text-white/70">{{
                                    scene?.release_date || '(empty)'
                                }}</span>
                            </div>
                            <div>
                                <span class="text-dim">New:</span>
                                <span class="text-lava ml-1">{{ porndbScene.date }}</span>
                            </div>
                        </div>
                    </div>
                </label>

                <!-- Thumbnail -->
                <label
                    v-if="porndbScene.image || porndbScene.poster"
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
                                    :src="(porndbScene.image || porndbScene.poster)!"
                                    :alt="porndbScene.title"
                                    class="h-full w-full object-cover"
                                />
                            </div>
                            <p class="text-dim text-[11px]">
                                Import scene thumbnail as scene thumbnail
                            </p>
                        </div>
                    </div>
                </label>

                <!-- Description (full width) -->
                <label
                    v-if="porndbScene.description"
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
                                    {{ scene?.description || '(empty)' }}
                                </p>
                            </div>
                            <div>
                                <span class="text-dim">New:</span>
                                <p class="text-lava mt-0.5 line-clamp-3">
                                    {{ porndbScene.description }}
                                </p>
                            </div>
                        </div>
                    </div>
                </label>

                <!-- Performers (full width) -->
                <label
                    v-if="porndbScene.performers?.length"
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
                            Performers ({{ porndbScene.performers.length }})
                        </p>
                        <div class="mt-2 flex flex-wrap gap-2">
                            <div
                                v-for="performer in porndbScene.performers"
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
                    v-if="porndbScene.tags?.length"
                    class="border-border bg-surface flex items-start gap-3 rounded-lg border p-3
                        lg:col-span-2"
                >
                    <input
                        v-model="selectedFields.tags"
                        type="checkbox"
                        class="accent-lava mt-0.5 h-4 w-4 shrink-0 rounded"
                    />
                    <div class="min-w-0 flex-1">
                        <p class="text-dim text-xs font-medium">
                            Tags ({{ porndbScene.tags.length }})
                        </p>
                        <div class="mt-2 flex flex-wrap gap-1">
                            <span
                                v-for="tag in porndbScene.tags.slice(0, 15)"
                                :key="tag.id"
                                class="text-dim rounded bg-white/5 px-1.5 py-0.5 text-[10px]"
                            >
                                {{ tag.name }}
                            </span>
                            <span v-if="porndbScene.tags.length > 15" class="text-dim text-[10px]">
                                +{{ porndbScene.tags.length - 15 }} more
                            </span>
                        </div>
                    </div>
                </label>

                <!-- Markers (full width) -->
                <label
                    v-if="porndbScene.markers?.length"
                    class="border-border bg-surface flex items-start gap-3 rounded-lg border p-3
                        lg:col-span-2"
                >
                    <input
                        v-model="selectedFields.markers"
                        type="checkbox"
                        class="accent-lava mt-0.5 h-4 w-4 shrink-0 rounded"
                    />
                    <div class="min-w-0 flex-1">
                        <p class="text-xs font-medium text-white">
                            Markers ({{ porndbScene.markers.length }})
                        </p>
                        <div class="mt-2 flex flex-wrap gap-1.5">
                            <span
                                v-for="marker in porndbScene.markers.slice(0, 10)"
                                :key="marker.id"
                                class="border-border flex items-center gap-1.5 rounded-full border
                                    px-2 py-0.5 text-[11px] text-white"
                            >
                                <span class="text-lava font-mono text-[10px]">
                                    {{ formatTime(marker.start_time) }}
                                </span>
                                {{ marker.title }}
                            </span>
                            <span
                                v-if="porndbScene.markers.length > 10"
                                class="text-dim text-[10px]"
                            >
                                +{{ porndbScene.markers.length - 10 }} more
                            </span>
                        </div>
                        <p class="text-dim mt-1.5 text-[10px]">
                            Import scene markers as scene bookmarks
                        </p>
                    </div>
                </label>
            </div>
        </div>

        <!-- Actions -->
        <div class="flex justify-end gap-2 pt-2">
            <button
                class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors hover:text-white"
                @click="emit('close')"
            >
                Cancel
            </button>
            <button
                :disabled="!Object.values(selectedFields).some(Boolean)"
                class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs font-semibold
                    text-white transition-all disabled:cursor-not-allowed disabled:opacity-40"
                @click="emit('apply', { ...selectedFields })"
            >
                Apply Selected
            </button>
        </div>
    </div>
</template>
