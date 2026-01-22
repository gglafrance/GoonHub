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
    <header class="bg-secondary/30 backdrop-blur-md sticky top-0 z-50 border-b border-white/5">
        <div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <div class="flex h-16 items-center justify-between">
                <NuxtLink to="/" class="flex items-center gap-2">
                    <h1 class="text-2xl font-extrabold tracking-tight text-white">
                        Goon<span class="text-neon-green">Hub</span>
                    </h1>
                </NuxtLink>

                <div v-if="authStore.user && authStore.token" class="flex items-center gap-4">
                    <div class="flex items-center gap-3">
                        <div class="text-right">
                            <div class="text-sm font-medium text-white">
                                {{ authStore.user.username }}
                            </div>
                            <div
                                class="text-xs uppercase tracking-wider"
                                :class="
                                    authStore.user.role === 'admin'
                                        ? 'text-neon-green'
                                        : 'text-gray-400'
                                "
                            >
                                {{ authStore.user.role }}
                            </div>
                        </div>
                        <div class="flex h-10 w-10 items-center justify-center rounded-full bg-black/50 border border-white/10">
                            <span class="text-lg font-bold text-white">
                                {{ authStore.user.username.charAt(0).toUpperCase() }}
                            </span>
                        </div>
                    </div>

                    <button
                        @click="handleLogout"
                        class="bg-neon-red/10 hover:bg-neon-red/20 rounded-lg px-3 py-2 text-sm font-medium text-neon-red transition-colors"
                    >
                        Logout
                    </button>
                </div>
            </div>
        </div>
    </header>
</template>
