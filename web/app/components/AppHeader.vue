<script setup lang="ts">
const authStore = useAuthStore();
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

                    <div v-if="authStore.isAuthenticated" class="flex items-center gap-3">
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

                        <NuxtLink
                            to="/settings"
                            class="border-border text-dim hover:border-lava/30 hover:text-lava flex
                                h-7 w-7 items-center justify-center rounded-md border
                                transition-all"
                        >
                            <Icon name="heroicons:cog-6-tooth" size="16" />
                        </NuxtLink>

                        <button
                            @click="authStore.logout()"
                            class="border-border text-dim hover:border-lava/30 hover:text-lava
                                rounded-md border px-2.5 py-1 text-[11px] font-medium
                                transition-all"
                        >
                            Logout
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
                <div class="flex h-9 items-center gap-1">
                    <NuxtLink
                        to="/search"
                        class="text-dim flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[11px]
                            font-medium transition-all hover:bg-white/5 hover:text-white"
                        active-class="!text-lava bg-lava/10"
                    >
                        <Icon name="heroicons:magnifying-glass" size="14" />
                        <span>Search</span>
                    </NuxtLink>

                    <NuxtLink
                        to="/history"
                        class="text-dim flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[11px]
                            font-medium transition-all hover:bg-white/5 hover:text-white"
                        active-class="!text-lava bg-lava/10"
                    >
                        <Icon name="heroicons:clock" size="14" />
                        <span>History</span>
                    </NuxtLink>
                </div>
            </div>
        </nav>
    </div>
</template>
