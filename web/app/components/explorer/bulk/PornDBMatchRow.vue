<script setup lang="ts">
import type { BulkMatchResult } from '~/types/bulk-match';

const props = defineProps<{
    result: BulkMatchResult;
}>();

const emit = defineEmits<{
    manualSearch: [sceneId: number];
    removeMatch: [sceneId: number];
}>();

const { formatDuration } = useFormatter();

const confidenceColor = computed(() => {
    if (!props.result.confidence) return 'text-dim';
    const total = props.result.confidence.total;
    if (total >= 80) return 'text-emerald-400';
    if (total >= 50) return 'text-amber-400';
    return 'text-red-400';
});

const confidenceBgColor = computed(() => {
    if (!props.result.confidence) return 'bg-white/5';
    const total = props.result.confidence.total;
    if (total >= 80) return 'bg-emerald-500/20';
    if (total >= 50) return 'bg-amber-500/20';
    return 'bg-red-500/20';
});

const statusIcon = computed(() => {
    switch (props.result.status) {
        case 'searching':
            return 'svg-spinners:90-ring-with-bg';
        case 'matched':
            return 'heroicons:check-circle';
        case 'no-match':
            return 'heroicons:question-mark-circle';
        case 'skipped':
            return 'heroicons:forward';
        case 'removed':
            return 'heroicons:x-circle';
        case 'applying':
            return 'svg-spinners:90-ring-with-bg';
        case 'applied':
            return 'heroicons:check-badge';
        case 'failed':
            return 'heroicons:exclamation-triangle';
        default:
            return 'heroicons:clock';
    }
});

const statusColor = computed(() => {
    switch (props.result.status) {
        case 'matched':
            return 'text-emerald-400';
        case 'applied':
            return 'text-emerald-400';
        case 'no-match':
        case 'removed':
            return 'text-amber-400';
        case 'failed':
            return 'text-red-400';
        default:
            return 'text-dim';
    }
});
</script>

<template>
    <div class="border-border bg-surface flex gap-4 rounded-lg border p-3">
        <!-- Local Scene -->
        <div class="flex min-w-0 flex-1 gap-3">
            <!-- Thumbnail -->
            <div class="bg-void aspect-video h-20 shrink-0 overflow-hidden rounded">
                <img
                    v-if="result.localScene.id"
                    :src="`/thumbnails/${result.localScene.id}`"
                    :alt="result.localScene.title"
                    class="h-full w-full object-cover"
                />
                <div v-else class="text-dim flex h-full w-full items-center justify-center">
                    <Icon name="heroicons:film" size="24" />
                </div>
            </div>

            <!-- Info -->
            <div class="flex min-w-0 flex-1 flex-col justify-center">
                <p class="truncate text-sm font-medium text-white" :title="result.localScene.title">
                    {{ result.localScene.title }}
                </p>
                <p
                    class="text-dim mt-0.5 truncate text-[11px]"
                    :title="result.localScene.original_filename"
                >
                    {{ result.localScene.original_filename }}
                </p>
                <div
                    class="text-dim mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-[10px]"
                >
                    <span v-if="result.localScene.duration">
                        {{ formatDuration(result.localScene.duration) }}
                    </span>
                    <span v-if="result.localScene.studio">
                        {{ result.localScene.studio }}
                    </span>
                    <span v-if="result.localScene.actors?.length">
                        {{ result.localScene.actors.join(', ') }}
                    </span>
                </div>
            </div>
        </div>

        <!-- Arrow -->
        <div class="flex shrink-0 items-center px-2">
            <Icon name="heroicons:arrow-right" size="20" class="text-dim" />
        </div>

        <!-- Match Result -->
        <div class="flex min-w-0 flex-1 gap-3">
            <!-- Searching -->
            <template v-if="result.status === 'searching'">
                <div class="flex flex-1 items-center justify-center">
                    <LoadingSpinner />
                </div>
            </template>

            <!-- Matched -->
            <template v-else-if="result.status === 'matched' && result.match">
                <!-- PornDB Thumbnail -->
                <div class="bg-void aspect-video h-20 shrink-0 overflow-hidden rounded">
                    <img
                        v-if="result.match.image || result.match.poster"
                        :src="result.match.image || result.match.poster"
                        :alt="result.match.title"
                        class="h-full w-full object-cover"
                    />
                    <div v-else class="text-dim flex h-full w-full items-center justify-center">
                        <Icon name="heroicons:film" size="24" />
                    </div>
                </div>

                <!-- PornDB Info -->
                <div class="flex min-w-0 flex-1 flex-col justify-center">
                    <div class="flex items-start gap-2">
                        <p
                            class="flex-1 truncate text-sm font-medium text-white"
                            :title="result.match.title"
                        >
                            {{ result.match.title }}
                        </p>
                        <!-- Confidence Badge -->
                        <div
                            v-if="result.confidence"
                            :class="[confidenceBgColor, confidenceColor]"
                            class="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold"
                            :title="`Title: ${result.confidence.titleScore}/30 | Actors: ${result.confidence.actorScore}/30 | Studio: ${result.confidence.studioScore}/20 | Duration: ${result.confidence.durationScore ?? 0}/20`"
                        >
                            {{ result.confidence.total }}%
                        </div>
                    </div>
                    <div
                        class="text-dim mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5
                            text-[10px]"
                    >
                        <span v-if="result.match.site?.name">
                            {{ result.match.site.name }}
                        </span>
                        <span v-if="result.match.performers?.length">
                            {{ result.match.performers.map((p) => p.name).join(', ') }}
                        </span>
                        <span v-if="result.match.date">
                            {{ result.match.date }}
                        </span>
                    </div>
                    <!-- Actions -->
                    <div class="mt-1.5 flex gap-2">
                        <button
                            class="text-dim text-[10px] transition-colors hover:text-white"
                            @click="emit('manualSearch', result.sceneId)"
                        >
                            Explore more
                        </button>
                        <button
                            class="text-[10px] text-red-400 transition-colors hover:text-red-300"
                            @click="emit('removeMatch', result.sceneId)"
                        >
                            Unmatch
                        </button>
                    </div>
                </div>
            </template>

            <!-- No Match / Removed -->
            <template v-else-if="result.status === 'no-match' || result.status === 'removed'">
                <div class="flex flex-1 flex-col items-center justify-center gap-2">
                    <p class="text-dim text-xs">
                        {{ result.status === 'removed' ? 'Match removed' : 'No match found' }}
                    </p>
                    <button
                        class="text-lava hover:text-lava-glow text-xs font-medium transition-colors"
                        @click="emit('manualSearch', result.sceneId)"
                    >
                        Search manually
                    </button>
                </div>
            </template>

            <!-- Skipped -->
            <template v-else-if="result.status === 'skipped'">
                <div class="flex flex-1 flex-col items-center justify-center">
                    <p class="text-dim text-xs">Already matched</p>
                </div>
            </template>

            <!-- Applying -->
            <template v-else-if="result.status === 'applying'">
                <div class="flex flex-1 items-center justify-center gap-2">
                    <LoadingSpinner />
                    <span class="text-dim text-xs">Applying...</span>
                </div>
            </template>

            <!-- Applied -->
            <template v-else-if="result.status === 'applied'">
                <div class="flex flex-1 flex-col items-center justify-center">
                    <Icon name="heroicons:check-circle" size="24" class="text-emerald-400" />
                    <p class="mt-1 text-xs text-emerald-400">Applied</p>
                </div>
            </template>

            <!-- Failed -->
            <template v-else-if="result.status === 'failed'">
                <div class="flex flex-1 flex-col items-center justify-center">
                    <Icon name="heroicons:exclamation-triangle" size="24" class="text-red-400" />
                    <p class="mt-1 text-xs text-red-400">Failed</p>
                    <p v-if="result.error" class="text-dim mt-0.5 text-[10px]">
                        {{ result.error }}
                    </p>
                </div>
            </template>

            <!-- Pending -->
            <template v-else>
                <div class="flex flex-1 items-center justify-center">
                    <Icon name="heroicons:clock" size="20" class="text-dim" />
                </div>
            </template>
        </div>

        <!-- Status indicator -->
        <div class="flex shrink-0 items-center">
            <Icon :name="statusIcon" size="18" :class="statusColor" />
        </div>
    </div>
</template>
