<script setup lang="ts">
import { useAuth } from '~/composables/useAuth';
import { useAuthStore } from '~/stores/auth';

const authComposable = useAuth();
const authStore = useAuthStore();

const handleLogout = () => {
    authComposable.logout();
};

definePageMeta({
    title: 'GoonHub',
});
</script>

<template>
    <header class="border-border bg-void/80 sticky top-0 z-50 border-b backdrop-blur-md">
        <div class="mx-auto max-w-[1600px] px-4 sm:px-5">
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

                <div v-if="authStore.user && authStore.token" class="flex items-center gap-3">
                    <div class="flex items-center gap-2">
                        <div
                            class="border-border bg-panel flex h-7 w-7 items-center justify-center
                                rounded-full border"
                        >
                            <span class="text-xs font-semibold text-white">
                                {{ authStore.user.username.charAt(0).toUpperCase() }}
                            </span>
                        </div>
                        <div class="hidden sm:block">
                            <div class="text-xs font-medium text-white">
                                {{ authStore.user.username }}
                            </div>
                            <div
                                class="font-mono text-[10px] tracking-wider uppercase"
                                :class="
                                    authStore.user.role === 'admin' ? 'text-emerald' : 'text-dim'
                                "
                            >
                                {{ authStore.user.role }}
                            </div>
                        </div>
                    </div>

                    <button
                        @click="handleLogout"
                        class="border-border text-dim hover:border-lava/30 hover:text-lava
                            rounded-md border px-2.5 py-1 text-[11px] font-medium transition-all"
                    >
                        Logout
                    </button>
                </div>
            </div>
        </div>
    </header>
</template>
