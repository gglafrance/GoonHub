<script setup lang="ts">
import type { MobileNav } from '~/utils/mobileNav';

// Mobile bottom tab bar (docs/NAVIGATION.md, "Mobile"): Home, the first pages of the user's
// layout, then Menu (the sheet with everything else). Steps aside while a text field has the
// focus (the on-screen keyboard is up). Its height is published as --app-bottom-nav-h.
const props = defineProps<{ nav: MobileNav }>();

const route = useRoute();
const { menuOpen, typing, toggleMenu } = useMobileNav();
const { badge } = useHeaderItems();

const shown = computed(() => !typing.value);

const homeActive = computed(() => route.path === '/' && !menuOpen.value);
const tabActive = (to: string | undefined) =>
    !!to && !menuOpen.value && isNavItemActive(route.path, to);
const menuActive = computed(
    () =>
        menuOpen.value || mobileMenuPages(props.nav).some((to) => isNavItemActive(route.path, to)),
);

// A dot on Menu when an item inside has a badge (unread messages, requests, public lobbies)
const menuBadge = computed(() =>
    [...props.nav.sections.flatMap((s) => s.items), ...props.nav.social].some(
        (i) => badge(i.id) !== null,
    ),
);

const BAR_HEIGHT = 'calc(3.5rem + env(safe-area-inset-bottom))';
watch(
    shown,
    (v) => {
        if (import.meta.client) setChromeVar('--app-bottom-nav-h', v ? BAR_HEIGHT : '0px');
    },
    { immediate: true },
);
onBeforeUnmount(() => setChromeVar('--app-bottom-nav-h', null));

const tabClass = `relative flex min-w-0 flex-1 flex-col items-center justify-center gap-0.5
    text-[10px] font-medium leading-none transition-colors select-none`;
</script>

<template>
    <nav
        v-show="shown"
        class="border-border bg-void/90 fixed inset-x-0 bottom-0 z-[45] border-t
            pb-[env(safe-area-inset-bottom)] backdrop-blur-md"
        aria-label="Main"
    >
        <div
            class="flex h-14 items-stretch pr-[env(safe-area-inset-right)]
                pl-[env(safe-area-inset-left)]"
        >
            <NuxtLink
                to="/"
                :class="[tabClass, homeActive ? 'text-lava' : 'text-dim']"
                :aria-current="homeActive ? 'page' : undefined"
            >
                <Icon :name="homeActive ? 'heroicons:home-solid' : 'heroicons:home'" size="22" />
                <span>Home</span>
            </NuxtLink>

            <NuxtLink
                v-for="item in nav.tabs"
                :key="item.id"
                :to="item.to!"
                :class="[tabClass, tabActive(item.to) ? 'text-lava' : 'text-dim']"
                :aria-current="tabActive(item.to) ? 'page' : undefined"
                :title="badge(item.id) ? `${item.label} (${badge(item.id)!.title})` : undefined"
            >
                <span class="relative">
                    <Icon :name="item.icon" size="22" />
                    <span
                        v-if="badge(item.id)"
                        class="bg-lava absolute -top-1 -right-2.5 flex h-3.5 min-w-3.5 items-center
                            justify-center rounded-full px-1 font-mono text-[9px] leading-none
                            font-semibold text-white"
                    >
                        {{ badge(item.id)!.text }}
                    </span>
                </span>
                <span class="max-w-full truncate px-1">{{ item.label }}</span>
            </NuxtLink>

            <button
                type="button"
                :class="[tabClass, menuActive ? 'text-lava' : 'text-dim']"
                :aria-expanded="menuOpen"
                aria-controls="mobile-menu-sheet"
                aria-label="Menu"
                @click="toggleMenu()"
            >
                <span class="relative">
                    <Icon :name="menuOpen ? 'heroicons:x-mark' : 'heroicons:bars-3'" size="22" />
                    <span
                        v-if="menuBadge && !menuOpen"
                        class="bg-lava absolute -top-0.5 -right-1 h-2 w-2 rounded-full
                            shadow-[0_0_6px_rgba(255,77,77,0.6)]"
                        aria-hidden="true"
                    />
                </span>
                <span>Menu</span>
            </button>
        </div>
    </nav>
</template>
