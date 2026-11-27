<script setup lang="ts">
import type { PresenceView } from '~/types/presence';

// One contact in the presence popover: avatar with its dot, name, and what they are doing.
// A watching entry offers "Watch this too", from the start or at their position (not synced).
const props = defineProps<{ view: PresenceView; now: number }>();
const emit = defineEmits<{ navigate: [] }>();

const activity = computed(() => props.view.activity);
const group = computed(() => presenceGroupOf(props.view));
const progress = computed(() =>
    activity.value ? presenceProgress(activity.value, props.now) : null,
);
const position = computed(() =>
    activity.value ? presencePosition(activity.value, props.now) : null,
);
const fromStart = computed(() =>
    activity.value ? presenceWatchPath(activity.value, false, props.now) : null,
);
const atPosition = computed(() =>
    activity.value ? presenceWatchPath(activity.value, true, props.now) : null,
);
</script>

<template>
    <div class="flex items-start gap-2.5 px-4 py-2">
        <NuxtLink
            :to="profilePath(view.user.username)"
            class="relative mt-0.5 shrink-0"
            @click="emit('navigate')"
        >
            <SocialUserAvatar :username="view.user.username" />
            <SocialPresenceDot :user-id="view.user.id" />
        </NuxtLink>
        <div class="min-w-0 flex-1">
            <NuxtLink
                :to="profilePath(view.user.username)"
                class="hover:text-lava block truncate text-xs font-medium text-white
                    transition-colors"
                @click="emit('navigate')"
            >
                {{ view.user.username }}
            </NuxtLink>

            <template v-if="activity && group === 'watching'">
                <div v-if="activity.scene" class="mt-1 flex gap-2">
                    <div
                        class="border-border bg-surface relative h-9 w-16 shrink-0 overflow-hidden
                            rounded border"
                    >
                        <img
                            v-if="activity.scene.thumbnail_url"
                            :src="activity.scene.thumbnail_url"
                            :alt="activity.scene.title"
                            class="h-full w-full object-cover"
                            loading="lazy"
                        />
                        <div
                            v-if="progress !== null"
                            class="absolute inset-x-0 bottom-0 h-0.5 bg-white/10"
                        >
                            <div class="bg-lava h-full" :style="{ width: `${progress}%` }" />
                        </div>
                    </div>
                    <div class="min-w-0 flex-1">
                        <div
                            class="truncate text-[11px] text-white/80"
                            :title="activityLabel(activity)"
                        >
                            {{ activityLabel(activity) }}
                        </div>
                        <div class="text-dim flex items-center gap-1 font-mono text-[10px]">
                            <Icon
                                :name="activity.paused ? 'heroicons:pause' : 'heroicons:play'"
                                size="10"
                            />
                            <span v-if="activity.paused">Paused</span>
                            <span v-if="position !== null">
                                {{ formatPresenceTime(position) }}
                                <template v-if="activity.scene.duration">
                                    / {{ formatPresenceTime(activity.scene.duration) }}
                                </template>
                            </span>
                        </div>
                    </div>
                </div>
                <div v-else class="text-dim mt-0.5 text-[11px]">Watching something</div>
                <div v-if="fromStart" class="mt-1.5 flex flex-wrap items-center gap-1.5">
                    <span class="text-dim text-[10px]">Watch this too:</span>
                    <NuxtLink
                        :to="fromStart"
                        class="border-border hover:border-lava/40 hover:text-lava rounded border
                            px-1.5 py-0.5 text-[10px] text-white/80 transition-colors"
                        @click="emit('navigate')"
                    >
                        From the start
                    </NuxtLink>
                    <NuxtLink
                        v-if="atPosition && atPosition !== fromStart && position !== null"
                        :to="atPosition"
                        class="border-border hover:border-lava/40 hover:text-lava rounded border
                            px-1.5 py-0.5 text-[10px] text-white/80 transition-colors"
                        @click="emit('navigate')"
                    >
                        At {{ formatPresenceTime(position) }}
                    </NuxtLink>
                </div>
            </template>
            <div
                v-else-if="group === 'away'"
                class="mt-0.5 flex min-w-0 items-center gap-1 text-[11px]"
            >
                <span class="shrink-0 text-amber-400/80">Away</span>
                <template v-if="activity">
                    <span class="text-dim shrink-0">&middot;</span>
                    <NuxtLink
                        v-if="fromStart"
                        :to="fromStart"
                        class="text-dim hover:text-lava truncate transition-colors"
                        :title="awayActivityLabel(activity)"
                        @click="emit('navigate')"
                    >
                        {{ awayActivityLabel(activity) }}
                    </NuxtLink>
                    <span v-else class="text-dim truncate">{{ awayActivityLabel(activity) }}</span>
                </template>
            </div>
            <div v-else-if="view.status === 'online'" class="text-emerald/80 mt-0.5 text-[11px]">
                Online
            </div>
            <div v-else class="text-dim mt-0.5 text-[11px]">
                <template v-if="view.last_seen_at">
                    Last seen <NuxtTime :datetime="view.last_seen_at" relative />
                </template>
                <template v-else>Offline</template>
            </div>
        </div>
    </div>
</template>
