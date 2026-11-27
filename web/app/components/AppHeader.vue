<script setup lang="ts">
const authStore = useAuthStore();
const perms = useMyPermissions();
const socialStore = useSocialStore();
const { shortcutsVisible, open: openPalette, modifierLabel } = useCommandPalette();
const setShortcutsVisible = (value: boolean) => {
    shortcutsVisible.value = value;
};
const { enabled: safeModeEnabled, toggle: toggleSafeMode } = useSafeMode();
</script>

<template>
    <div class="sticky top-0 z-50">
        <!-- Main Header -->
        <header class="border-border bg-void/80 border-b backdrop-blur-md">
            <div class="mx-auto max-w-415 px-4 sm:px-5">
                <div class="flex h-12 items-center justify-between">
                    <NuxtLink to="/" class="group flex items-center gap-2">
                        <div
                            class="bg-lava/10 group-hover:bg-lava/20 flex h-7 w-7 items-center
                                justify-center rounded-md transition-colors"
                        >
                            <div class="bg-lava h-2 w-2 rounded-full"></div>
                        </div>
                        <h1 class="text-sm font-bold tracking-tight text-white">
                            GOON<span class="text-lava">HUB</span>
                        </h1>
                    </NuxtLink>

                    <div v-if="authStore.isAuthenticated" class="flex items-center gap-2 sm:gap-3">
                        <!-- Command palette: compact search field on desktop, icon on mobile -->
                        <button
                            class="border-border bg-panel/60 text-dim hover:border-lava/30 group
                                flex h-7 w-7 items-center justify-center gap-2 rounded-md border
                                transition-all hover:text-white sm:w-56 sm:justify-start sm:px-2
                                lg:w-64"
                            :title="`Search and commands (${modifierLabel}+K)`"
                            aria-label="Open the command palette"
                            @click="openPalette()"
                        >
                            <Icon
                                name="heroicons:magnifying-glass"
                                size="14"
                                class="group-hover:text-lava shrink-0 transition-colors"
                            />
                            <span class="hidden flex-1 truncate text-left text-[11px] sm:inline">
                                Search or jump to...
                            </span>
                            <kbd
                                class="border-border bg-surface hidden shrink-0 items-center gap-0.5
                                    rounded border px-1 py-px font-mono text-[10px] sm:inline-flex"
                            >
                                {{ modifierLabel }} K
                            </kbd>
                        </button>

                        <NuxtLink
                            :to="profilePath(authStore.user!.username)"
                            class="group flex items-center gap-2"
                            title="My profile"
                        >
                            <SocialUserAvatar
                                :username="authStore.user!.username"
                                :display-name="socialStore.me?.display_name"
                                :user-id="socialStore.me?.id"
                                :avatar-version="socialStore.me?.avatar_version"
                                size="xs"
                                class="group-hover:border-lava/50 !h-7 !w-7"
                            />
                            <div class="hidden sm:block">
                                <div class="group-hover:text-lava text-xs font-medium text-white">
                                    {{ socialStore.me?.display_name || authStore.user!.username }}
                                </div>
                                <div
                                    class="font-mono text-[10px] tracking-wider uppercase"
                                    :class="perms.isSuperuser.value ? 'text-emerald' : 'text-dim'"
                                >
                                    {{ authStore.user!.role }}
                                </div>
                            </div>
                        </NuxtLink>

                        <!-- Keyboard shortcuts - hidden on mobile -->
                        <button
                            class="border-border text-dim hover:border-lava/30 hover:text-lava
                                hidden h-7 w-7 items-center justify-center rounded-md border
                                transition-all sm:flex"
                            title="Keyboard shortcuts"
                            @click="setShortcutsVisible(true)"
                        >
                            <Icon name="heroicons:command-line" size="16" />
                        </button>

                        <button
                            class="border-border flex h-7 w-7 items-center justify-center rounded-md
                                border transition-all"
                            :class="
                                safeModeEnabled
                                    ? 'border-lava/30 text-lava'
                                    : 'text-dim hover:border-lava/30 hover:text-lava'
                            "
                            :title="safeModeEnabled ? 'Disable safe mode' : 'Enable safe mode'"
                            @click="toggleSafeMode"
                        >
                            <Icon
                                :name="safeModeEnabled ? 'heroicons:eye-slash' : 'heroicons:eye'"
                                size="16"
                            />
                        </button>

                        <HeaderHoverAudioControl />

                        <HeaderJobStatus v-if="perms.has('jobs:view_status')" />

                        <HeaderFriendsPresence />

                        <HeaderFriendsLink />

                        <NuxtLink
                            to="/settings"
                            class="border-border text-dim hover:border-lava/30 hover:text-lava flex
                                h-7 w-7 items-center justify-center rounded-md border
                                transition-all"
                        >
                            <Icon name="heroicons:cog-6-tooth" size="16" />
                        </NuxtLink>

                        <!-- Logout - icon only on mobile -->
                        <button
                            class="border-border text-dim hover:border-lava/30 hover:text-lava flex
                                h-7 w-7 items-center justify-center rounded-md border transition-all
                                sm:h-auto sm:w-auto sm:px-2.5 sm:py-1"
                            @click="authStore.logout()"
                        >
                            <Icon
                                name="heroicons:arrow-right-on-rectangle"
                                size="16"
                                class="sm:hidden"
                            />
                            <span class="ml-1.5 hidden text-[11px] font-medium sm:inline"
                                >Logout</span
                            >
                        </button>
                    </div>
                </div>
            </div>
        </header>

        <!-- Secondary nav bar: user pages, then the Manage menu -->
        <HeaderNavBar v-if="authStore.isAuthenticated" />

        <!-- Keyboard Shortcuts Modal -->
        <KeyboardShortcutsModal :visible="shortcutsVisible" @close="setShortcutsVisible(false)" />

        <!-- Command palette (Ctrl/Cmd+K) -->
        <CommandPalette v-if="authStore.isAuthenticated" />
    </div>
</template>
