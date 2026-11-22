<script setup lang="ts">
import type { SimilarScene } from '~/types/duplicates';

// A scene matching the watched one by fingerprint: card data, relation, matching segment
const props = defineProps<{
    scene: SimilarScene;
    currentSceneId: number;
}>();

const emit = defineEmits<{
    // Play the watched scene from this time (start of the matching part)
    seek: [time: number];
}>();

const { formatDuration, formatSize } = useFormatter();
const { relationClass } = useDuplicateRelation();

// A trashed scene cannot be opened (admins only get them)
const openable = computed(() => !props.scene.is_trashed);
const link = computed(() => similarSceneLink(props.scene));
const NuxtLinkComponent = resolveComponent('NuxtLink');
</script>

<template>
    <div
        class="border-border bg-surface hover:border-border-hover flex gap-3 rounded-lg border p-2
            transition-colors"
    >
        <!-- Thumbnail -->
        <component
            :is="openable ? NuxtLinkComponent : 'div'"
            :to="openable ? link : undefined"
            class="group/thumb relative aspect-video w-32 shrink-0 overflow-hidden rounded
                bg-black/50"
            :class="{ grayscale: scene.is_trashed }"
            :title="openable ? 'Open at the matching part' : 'In trash'"
        >
            <img
                v-if="scene.thumbnail_path"
                :src="`/thumbnails/${scene.id}?size=sm`"
                class="h-full w-full object-cover"
                :alt="scene.title"
                loading="lazy"
            />
            <div v-else class="flex h-full w-full items-center justify-center">
                <Icon name="heroicons:film" size="18" class="text-dim" />
            </div>
            <span
                class="absolute right-1 bottom-1 rounded bg-black/70 px-1 font-mono text-[9px]
                    text-white"
            >
                {{ formatDuration(scene.duration) }}
            </span>
            <span
                v-if="openable"
                class="bg-lava/30 absolute inset-0 flex items-center justify-center opacity-0
                    transition-opacity group-hover/thumb:opacity-100"
            >
                <Icon name="heroicons:play-solid" size="16" class="text-white" />
            </span>
        </component>

        <div class="min-w-0 flex-1 space-y-1 text-[11px]">
            <div class="flex items-center gap-1.5">
                <NuxtLink
                    v-if="openable"
                    :to="link"
                    class="hover:text-lava truncate font-medium text-white transition-colors"
                    :title="scene.title"
                >
                    {{ scene.title }}
                </NuxtLink>
                <span v-else class="text-dim truncate font-medium" :title="scene.title">
                    {{ scene.title }}
                </span>
                <span
                    v-if="scene.is_trashed"
                    class="text-dim shrink-0 rounded-full bg-white/5 px-1.5 py-0.5 text-[9px]
                        font-medium"
                >
                    Trashed
                </span>
            </div>

            <div class="flex flex-wrap items-center gap-1.5">
                <span
                    class="inline-flex items-center rounded-full px-1.5 py-0.5 text-[9px]
                        font-semibold whitespace-nowrap"
                    :class="relationClass(scene.relation)"
                    :title="similarRelationTitle(scene, currentSceneId)"
                >
                    {{ similarRelationLabel(scene, currentSceneId) }}
                </span>
                <span class="text-dim" title="Match confidence">
                    {{ formatConfidence(scene.confidence_score) }}
                </span>
                <span
                    class="text-dim inline-flex items-center"
                    :title="scene.match_type === 'visual' ? 'Visual match' : 'Audio match'"
                >
                    <Icon
                        :name="
                            scene.match_type === 'visual'
                                ? 'heroicons:eye'
                                : 'heroicons:musical-note'
                        "
                        size="11"
                    />
                </span>
                <span class="text-dim">·</span>
                <span class="text-dim" :title="`${scene.width}x${scene.height}`">
                    {{ resolutionLabel(scene.height) || '-' }}
                </span>
                <span class="text-dim">·</span>
                <span class="text-dim">{{ formatSize(scene.size) }}</span>
            </div>

            <div
                v-if="scene.segment"
                class="text-dim font-mono text-[10px]"
                :title="describeSimilarSegment(scene.segment)"
            >
                matches
                <button
                    class="text-lava/80 hover:text-lava transition-colors"
                    title="Play this scene from here"
                    @click="emit('seek', scene.segment.this_start)"
                >
                    {{ formatSegmentRange(scene.segment.this_start, scene.segment.this_end) }}
                </button>
                here,
                <NuxtLink
                    v-if="openable"
                    :to="link"
                    class="hover:text-lava text-white/80 transition-colors"
                    title="Open the other scene there"
                >
                    {{ formatSegmentRange(scene.segment.other_start, scene.segment.other_end) }}
                </NuxtLink>
                <span v-else>
                    {{ formatSegmentRange(scene.segment.other_start, scene.segment.other_end) }}
                </span>
                there
            </div>
            <div v-else class="text-dim text-[10px]">No matching segment recorded</div>
        </div>
    </div>
</template>
