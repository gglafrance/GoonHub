<script setup lang="ts">
// The header, laid out from the user's layout (docs/NAVIGATION.md, stores/navLayout).
// From md: the top bar (by default the logo, then the search and account icons on the right)
// and the nav row (by default the user pages, then the Manage folder). Below md: a compact top
// bar, a bottom tab bar and a Menu sheet built from the same layout (utils/mobileNav.ts).
// Items the user lacks the permission for are never shown. Signed out, only the logo.
const authStore = useAuthStore();
const perms = useMyPermissions();
const navLayoutStore = useNavLayoutStore();
const { shortcutsVisible } = useCommandPalette();
const { isMobile, menuOpen } = useMobileNav();
const setShortcutsVisible = (value: boolean) => {
    shortcutsVisible.value = value;
};

const resolved = computed(() => resolveNavLayout(navLayoutStore.layout, perms.has));
const showNavRow = computed(() => !isNavBarEmpty(resolved.value.nav));
const mobileNav = computed(() => buildMobileNav(resolved.value));
const mobile = computed(() => authStore.isAuthenticated && isMobile.value);

// Phones: the top bar slides away while scrolling down and comes back on the way up (the tab
// bar stays). Never while the Menu sheet is open, nor near the top of the page.
const SCROLL_DELTA = 6;
const hidden = ref(false);
const headerHeight = ref(0);
let lastY = 0;
function onScroll() {
    const y = window.scrollY;
    const dy = y - lastY;
    if (!mobile.value || menuOpen.value || y < headerHeight.value) {
        hidden.value = false;
        lastY = y;
    } else if (Math.abs(dy) > SCROLL_DELTA) {
        hidden.value = dy > 0;
        lastY = y;
    }
}
watch([mobile, menuOpen], () => onScroll());

// Height of the sticky header (0 while slid away), for the pages and bars that stick right
// under it
const headerRef = ref<HTMLElement | null>(null);
let resizeObserver: ResizeObserver | null = null;
let stopTracking: (() => void) | null = null;

watch([headerHeight, hidden], ([h, isHidden]) => {
    if (h > 0) setChromeVar('--app-header-h', isHidden ? '0px' : `${h}px`);
});

onMounted(() => {
    stopTracking = startMobileNavTracking();
    window.addEventListener('scroll', onScroll, { passive: true });
    if (headerRef.value) {
        resizeObserver = new ResizeObserver(([entry]) => {
            if (!entry) return;
            const h = entry.borderBoxSize?.[0]?.blockSize ?? entry.contentRect.height;
            headerHeight.value = Math.round(h);
        });
        resizeObserver.observe(headerRef.value);
    }
});

onBeforeUnmount(() => {
    stopTracking?.();
    window.removeEventListener('scroll', onScroll);
    resizeObserver?.disconnect();
    setChromeVar('--app-header-h', null);
});
</script>

<template>
    <div
        ref="headerRef"
        class="sticky top-0 z-50 pt-[env(safe-area-inset-top)] transition-transform duration-200"
        :class="[
            mobile ? 'bg-void/85 backdrop-blur-md' : '',
            { '-translate-y-full': hidden && mobile },
        ]"
    >
        <!-- Phones: compact top bar (the pages are in the bottom tab bar and the Menu sheet) -->
        <HeaderMobileTopBar v-if="mobile" :items="mobileNav.quick" />

        <template v-else>
            <!-- Main Header -->
            <header class="border-border bg-void/80 border-b backdrop-blur-md">
                <div class="mx-auto max-w-415 px-4 sm:px-5">
                    <HeaderLayoutBar
                        v-if="authStore.isAuthenticated"
                        bar="top"
                        :zones="resolved.top"
                    />
                    <div v-else class="flex h-12 items-center">
                        <HeaderLogo />
                    </div>
                </div>
            </header>

            <!-- Secondary nav bar -->
            <HeaderNavBar v-if="authStore.isAuthenticated && showNavRow" :zones="resolved.nav" />
        </template>

        <!-- Live badges, toasts and the layout, whatever is shown -->
        <HeaderLiveSources v-if="authStore.isAuthenticated" />
    </div>

    <!-- Outside the header: its slide (a transform) would anchor their fixed overlays to it -->
    <KeyboardShortcutsModal :visible="shortcutsVisible" @close="setShortcutsVisible(false)" />
    <CommandPalette v-if="authStore.isAuthenticated" />

    <!-- Phones: bottom tab bar and the Menu sheet -->
    <template v-if="mobile">
        <HeaderMobileTabBar :nav="mobileNav" />
        <HeaderMobileMenuSheet :nav="mobileNav" />
    </template>
</template>
