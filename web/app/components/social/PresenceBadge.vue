<script setup lang="ts">
// Presence of a user in words (profile header): watching (with a link), online, away (with
// what was left open, e.g. "Paused on X"), or last seen. Nothing when hidden by the user's privacy. Loaded once, then kept live by the store.
const props = defineProps<{ userId: number }>();

const presenceStore = usePresenceStore();
const view = computed(() => presenceStore.viewOf(props.userId));
const group = computed(() => (view.value ? presenceGroupOf(view.value) : null));
const watchPath = computed(() =>
    view.value?.activity ? presenceWatchPath(view.value.activity, false, Date.now()) : null,
);

async function load() {
    if (presenceStore.views[props.userId]) return;
    try {
        presenceStore.apply(await useApiPresence().getUserPresence(props.userId));
    } catch {
        // no badge
    }
}

onMounted(load);
watch(() => props.userId, load);
</script>

<template>
    <div v-if="view && group" class="flex min-w-0 items-center gap-1.5 text-[11px]">
        <span
            class="h-2 w-2 shrink-0 rounded-full"
            :class="{
                'bg-lava': group === 'watching',
                'bg-emerald': group === 'online',
                'bg-amber-400': group === 'away',
                'bg-white/25': group === 'offline',
            }"
        />
        <template v-if="group === 'watching' && view.activity">
            <span class="text-dim shrink-0">Watching</span>
            <NuxtLink
                v-if="watchPath"
                :to="watchPath"
                class="hover:text-lava truncate text-white transition-colors"
            >
                {{ activityLabel(view.activity) }}
            </NuxtLink>
            <span v-else class="text-dim">something</span>
        </template>
        <span v-else-if="group === 'online'" class="text-emerald">Online</span>
        <template v-else-if="group === 'away'">
            <span class="shrink-0 text-amber-400">Away</span>
            <template v-if="view.activity">
                <span class="text-dim shrink-0">&middot;</span>
                <NuxtLink
                    v-if="watchPath"
                    :to="watchPath"
                    class="text-dim hover:text-lava truncate transition-colors"
                >
                    {{ awayActivityLabel(view.activity) }}
                </NuxtLink>
                <span v-else class="text-dim truncate">
                    {{ awayActivityLabel(view.activity) }}
                </span>
            </template>
        </template>
        <span v-else class="text-dim">
            Offline<template v-if="view.last_seen_at">
                , last seen <NuxtTime :datetime="view.last_seen_at" relative />
            </template>
        </span>
    </div>
</template>
