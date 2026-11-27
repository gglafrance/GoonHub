<script setup lang="ts">
import type { NavLayout } from '~/types/navLayout';

// What the edited layout gives on phones (docs/NAVIGATION.md, "Mobile"): the quick icons of the
// top bar and the bottom tab bar (Home, the first pages of the layout, Menu).
const props = defineProps<{ layout: NavLayout }>();

const perms = useMyPermissions();
const tabPages = MOBILE_TAB_PAGES;
const nav = computed(() => buildMobileNav(resolveNavLayout(props.layout, perms.has)));
</script>

<template>
    <div class="glass-panel p-5">
        <div class="flex items-center gap-1.5">
            <Icon name="heroicons:device-phone-mobile" size="14" class="text-lava" />
            <h3 class="text-sm font-semibold text-white">On phones</h3>
        </div>
        <p class="text-dim mt-0.5 text-[11px]">
            The bottom bar shows Home, the first {{ tabPages }} pages of this layout (pages outside
            folders first), then Menu with everything else. Reorder the pages to choose your tabs;
            hidden items stay hidden.
        </p>
        <div class="mt-3 flex flex-wrap items-center gap-4">
            <div
                class="border-border bg-void/60 flex h-12 w-full max-w-[22rem] items-stretch
                    rounded-lg border"
                aria-label="Bottom tab bar preview"
            >
                <div
                    v-for="tab in [
                        { id: 'home', label: 'Home', icon: 'heroicons:home' },
                        ...nav.tabs,
                        { id: 'menu', label: 'Menu', icon: 'heroicons:bars-3' },
                    ]"
                    :key="tab.id"
                    class="text-dim flex min-w-0 flex-1 flex-col items-center justify-center gap-0.5
                        text-[9px]"
                    :class="{ 'text-lava': tab.id === 'home' }"
                >
                    <Icon :name="tab.icon" size="16" />
                    <span class="max-w-full truncate px-0.5">{{ tab.label }}</span>
                </div>
            </div>
            <div v-if="nav.quick.length > 0" class="text-dim flex items-center gap-2 text-[11px]">
                Top bar:
                <span
                    v-for="item in nav.quick"
                    :key="item.id"
                    class="border-border flex h-7 w-7 items-center justify-center rounded-md border"
                    :title="item.label"
                >
                    <Icon :name="item.icon" size="14" />
                </span>
            </div>
        </div>
    </div>
</template>
