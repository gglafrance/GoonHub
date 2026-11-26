<script setup lang="ts">
// The two header bars, laid out from the user's layout (docs/NAVIGATION.md, stores/navLayout):
// the top bar (by default the logo, then the search and account icons on the right) and the
// nav row (by default the user pages, then the Manage folder). Items the user lacks the
// permission for are never shown. Signed out, only the logo.
const authStore = useAuthStore();
const perms = useMyPermissions();
const navLayoutStore = useNavLayoutStore();
const { shortcutsVisible } = useCommandPalette();
const setShortcutsVisible = (value: boolean) => {
    shortcutsVisible.value = value;
};

const resolved = computed(() => resolveNavLayout(navLayoutStore.layout, perms.has));
const showNavRow = computed(() => !isNavBarEmpty(resolved.value.nav));
</script>

<template>
    <div class="sticky top-0 z-50">
        <!-- Main Header -->
        <header class="border-border bg-void/80 border-b backdrop-blur-md">
            <div class="mx-auto max-w-415 px-4 sm:px-5">
                <HeaderLayoutBar v-if="authStore.isAuthenticated" bar="top" :zones="resolved.top" />
                <div v-else class="flex h-12 items-center">
                    <HeaderLogo />
                </div>
            </div>
        </header>

        <!-- Secondary nav bar -->
        <HeaderNavBar v-if="authStore.isAuthenticated && showNavRow" :zones="resolved.nav" />

        <!-- Live badges, toasts and the layout, whatever is shown -->
        <HeaderLiveSources v-if="authStore.isAuthenticated" />

        <!-- Keyboard Shortcuts Modal -->
        <KeyboardShortcutsModal :visible="shortcutsVisible" @close="setShortcutsVisible(false)" />

        <!-- Command palette (Ctrl/Cmd+K) -->
        <CommandPalette v-if="authStore.isAuthenticated" />
    </div>
</template>
