<script setup lang="ts">
const authStore = useAuthStore();
const showShortcuts = ref(false);
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
                        <div class="flex items-center gap-2">
                            <div
                                class="border-border bg-panel flex h-7 w-7 items-center
                                    justify-center rounded-full border"
                            >
                                <span class="text-xs font-semibold text-white">
                                    {{ authStore.user!.username.charAt(0).toUpperCase() }}
                                </span>
                            </div>
                            <div class="hidden sm:block">
                                <div class="text-xs font-medium text-white">
                                    {{ authStore.user!.username }}
                                </div>
                                <div
                                    class="font-mono text-[10px] tracking-wider uppercase"
                                    :class="
                                        authStore.user!.role === 'admin'
                                            ? 'text-emerald'
                                            : 'text-dim'
                                    "
                                >
                                    {{ authStore.user!.role }}
                                </div>
                            </div>
                        </div>

                        <!-- Keyboard shortcuts - hidden on mobile -->
                        <button
                            class="border-border text-dim hover:border-lava/30 hover:text-lava
                                hidden h-7 w-7 items-center justify-center rounded-md border
                                transition-all sm:flex"
                            title="Keyboard shortcuts"
                            @click="showShortcuts = true"
                        >
                            <Icon name="heroicons:command-line" size="16" />
                        </button>

                        <HeaderJobStatus />

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

        <!-- Secondary Nav Bar -->
        <nav
            v-if="authStore.isAuthenticated"
            class="border-border bg-void/60 border-b backdrop-blur-sm"
        >
            <div class="mx-auto max-w-415 px-4 sm:px-5">
                <div
                    class="flex h-10 items-center justify-between sm:h-9 sm:justify-start sm:gap-1"
                >
                    <NuxtLink
                        to="/search"
                        class="text-dim flex items-center justify-center rounded-md px-3 py-1.5
                            text-[11px] font-medium transition-all hover:bg-white/5 hover:text-white
                            sm:justify-start sm:gap-1.5 sm:px-2.5 sm:py-1"
                        active-class="!text-lava bg-lava/10"
                        title="Search"
                    >
                        <Icon
                            name="heroicons:magnifying-glass"
                            size="18"
                            class="sm:!h-3.5 sm:!w-3.5"
                        />
                        <span class="hidden sm:inline">Search</span>
                    </NuxtLink>

                    <NuxtLink
                        to="/history"
                        class="text-dim flex items-center justify-center rounded-md px-3 py-1.5
                            text-[11px] font-medium transition-all hover:bg-white/5 hover:text-white
                            sm:justify-start sm:gap-1.5 sm:px-2.5 sm:py-1"
                        active-class="!text-lava bg-lava/10"
                        title="History"
                    >
                        <Icon name="heroicons:clock" size="18" class="sm:!h-3.5 sm:!w-3.5" />
                        <span class="hidden sm:inline">History</span>
                    </NuxtLink>

                    <NuxtLink
                        to="/actors"
                        class="text-dim flex items-center justify-center rounded-md px-3 py-1.5
                            text-[11px] font-medium transition-all hover:bg-white/5 hover:text-white
                            sm:justify-start sm:gap-1.5 sm:px-2.5 sm:py-1"
                        active-class="!text-lava bg-lava/10"
                        title="Actors"
                    >
                        <Icon name="heroicons:user-group" size="18" class="sm:!h-3.5 sm:!w-3.5" />
                        <span class="hidden sm:inline">Actors</span>
                    </NuxtLink>

                    <NuxtLink
                        to="/studios"
                        class="text-dim flex items-center justify-center rounded-md px-3 py-1.5
                            text-[11px] font-medium transition-all hover:bg-white/5 hover:text-white
                            sm:justify-start sm:gap-1.5 sm:px-2.5 sm:py-1"
                        active-class="!text-lava bg-lava/10"
                        title="Studios"
                    >
                        <Icon
                            name="heroicons:building-office-2"
                            size="18"
                            class="sm:!h-3.5 sm:!w-3.5"
                        />
                        <span class="hidden sm:inline">Studios</span>
                    </NuxtLink>

                    <NuxtLink
                        to="/playlists"
                        class="text-dim flex items-center justify-center rounded-md px-3 py-1.5
                            text-[11px] font-medium transition-all hover:bg-white/5 hover:text-white
                            sm:justify-start sm:gap-1.5 sm:px-2.5 sm:py-1"
                        active-class="!text-lava bg-lava/10"
                        title="Playlists"
                    >
                        <Icon name="heroicons:queue-list" size="18" class="sm:!h-3.5 sm:!w-3.5" />
                        <span class="hidden sm:inline">Playlists</span>
                    </NuxtLink>

                    <NuxtLink
                        to="/explorer"
                        class="text-dim flex items-center justify-center rounded-md px-3 py-1.5
                            text-[11px] font-medium transition-all hover:bg-white/5 hover:text-white
                            sm:justify-start sm:gap-1.5 sm:px-2.5 sm:py-1"
                        active-class="!text-lava bg-lava/10"
                        title="Explorer"
                    >
                        <Icon name="heroicons:folder" size="18" class="sm:!h-3.5 sm:!w-3.5" />
                        <span class="hidden sm:inline">Explorer</span>
                    </NuxtLink>

                    <NuxtLink
                        to="/markers"
                        class="text-dim flex items-center justify-center rounded-md px-3 py-1.5
                            text-[11px] font-medium transition-all hover:bg-white/5 hover:text-white
                            sm:justify-start sm:gap-1.5 sm:px-2.5 sm:py-1"
                        active-class="!text-lava bg-lava/10"
                        title="Markers"
                    >
                        <Icon name="heroicons:bookmark" size="18" class="sm:!h-3.5 sm:!w-3.5" />
                        <span class="hidden sm:inline">Markers</span>
                    </NuxtLink>
                </div>
            </div>
        </nav>

        <!-- Keyboard Shortcuts Modal -->
        <KeyboardShortcutsModal :visible="showShortcuts" @close="showShortcuts = false" />
    </div>
</template>
