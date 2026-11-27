<script setup lang="ts">
import type { NavItemDef } from '~/utils/navRegistry';

// A row of the mobile Menu sheet: a page (friends, messages, settings) with its badge, a toggle
// (safe mode, preview sound) shown as a switch, the presence (expands the contacts online), or
// the logout.
const props = defineProps<{ item: NavItemDef }>();

const route = useRoute();
const presenceStore = usePresenceStore();
const { enabled: safeModeEnabled } = useSafeMode();
const { enabled: soundEnabled, volume } = useHoverAudio();
const { badge: badgeFor, route: routeFor, run } = useHeaderItems();

const to = computed(() => routeFor(props.item));
const badge = computed(() => badgeFor(props.item.id));
const active = computed(() => !!to.value && isNavItemActive(route.path, to.value));

const toggle = computed<boolean | null>(() => {
    if (props.item.id === 'safe_mode') return safeModeEnabled.value;
    if (props.item.id === 'preview_sound') return soundEnabled.value && volume.value > 0;
    return null;
});

const isPresence = computed(() => props.item.id === 'presence');
const presenceOpen = ref(false);
const groups = computed(() =>
    groupPresence(presenceStore.contacts).filter((g) => g.key !== 'offline'),
);
const now = ref(Date.now());
watch(presenceOpen, (open) => {
    if (open) now.value = Date.now();
});

const danger = computed(() => props.item.id === 'logout');
const rowClass = `flex min-h-12 w-full items-center gap-3 px-4 text-left text-[13px]
    font-medium transition-colors active:bg-white/5`;
</script>

<template>
    <!-- Presence: expands the contacts online -->
    <div v-if="isPresence">
        <button
            type="button"
            :class="[rowClass, 'text-white/85']"
            :aria-expanded="presenceOpen"
            @click="presenceOpen = !presenceOpen"
        >
            <Icon name="heroicons:signal" size="20" class="shrink-0 text-white/50" />
            <span class="flex-1">Friends online</span>
            <span class="text-dim flex items-center gap-1.5 font-mono text-[11px]">
                <span
                    class="h-1.5 w-1.5 rounded-full"
                    :class="
                        presenceStore.activeCount > 0
                            ? 'bg-emerald shadow-[0_0_6px_rgba(16,185,129,0.6)]'
                            : presenceStore.presentCount > 0
                              ? 'bg-amber-400'
                              : 'bg-white/20'
                    "
                />
                {{ presenceStore.presentCount }}
            </span>
            <Icon
                name="heroicons:chevron-down"
                size="16"
                class="text-dim transition-transform"
                :class="{ 'rotate-180': presenceOpen }"
            />
        </button>
        <div v-if="presenceOpen" class="border-border bg-void/40 border-y py-1">
            <div v-if="groups.length === 0" class="text-dim px-4 py-3 text-[11px]">
                Nobody online among your friends.
            </div>
            <div v-for="g in groups" :key="g.key" class="py-1">
                <div
                    class="px-4 pt-1 pb-0.5 text-[10px] font-medium tracking-wider text-white/40
                        uppercase"
                >
                    {{ g.label }} <span class="text-white/25">{{ g.views.length }}</span>
                </div>
                <SocialPresenceEntry v-for="v in g.views" :key="v.user.id" :view="v" :now="now" />
            </div>
        </div>
    </div>

    <!-- Toggles -->
    <button
        v-else-if="toggle !== null"
        type="button"
        role="switch"
        :aria-checked="toggle"
        :class="[rowClass, 'text-white/85']"
        @click="run(item)"
    >
        <Icon
            :name="
                item.id === 'safe_mode'
                    ? 'heroicons:eye-slash'
                    : toggle
                      ? 'heroicons:speaker-wave'
                      : 'heroicons:speaker-x-mark'
            "
            size="20"
            class="shrink-0"
            :class="toggle ? 'text-lava' : 'text-white/50'"
        />
        <span class="flex-1">{{ item.label }}</span>
        <span
            class="relative h-5 w-9 shrink-0 rounded-full border transition-colors"
            :class="toggle ? 'border-emerald/50 bg-emerald/80' : 'border-border bg-white/10'"
            aria-hidden="true"
        >
            <span
                class="absolute top-0.5 left-0.5 h-3.5 w-3.5 rounded-full bg-white shadow
                    transition-transform"
                :class="toggle ? 'translate-x-4' : ''"
            />
        </span>
    </button>

    <!-- Pages -->
    <NuxtLink
        v-else-if="to"
        :to="to"
        :class="[rowClass, active ? 'text-lava' : 'text-white/85']"
        :aria-current="active ? 'page' : undefined"
    >
        <Icon
            :name="item.icon"
            size="20"
            class="shrink-0"
            :class="active ? 'text-lava' : 'text-white/50'"
        />
        <span class="flex-1">{{ item.label }}</span>
        <span
            v-if="badge"
            class="bg-lava flex h-5 min-w-5 items-center justify-center rounded-full px-1.5
                font-mono text-[10px] font-semibold text-white"
            :aria-label="badge.title"
        >
            {{ badge.text }}
        </span>
        <Icon name="heroicons:chevron-right" size="16" class="text-dim" />
    </NuxtLink>

    <!-- Actions (logout) -->
    <button
        v-else
        type="button"
        :class="[rowClass, danger ? 'text-lava' : 'text-white/85']"
        @click="run(item)"
    >
        <Icon :name="item.icon" size="20" class="shrink-0" />
        <span class="flex-1">{{ item.label }}</span>
    </button>
</template>
