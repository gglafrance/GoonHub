<script setup lang="ts">
const authStore = useAuthStore();

useHead({ title: 'Login' });

const username = ref('');
const password = ref('');
const isLoading = ref(false);
const error = ref('');

const route = useRoute();

const handleLogin = async () => {
    isLoading.value = true;
    error.value = '';

    try {
        await authStore.login(username.value, password.value);
        const redirect = route.query.redirect as string;
        navigateTo(redirect || '/');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Invalid credentials';
    } finally {
        isLoading.value = false;
    }
};
</script>

<template>
    <div class="flex min-h-screen items-center justify-center px-4">
        <div class="w-full max-w-sm">
            <!-- Logo -->
            <div class="mb-8 text-center">
                <div
                    class="bg-lava/10 glow-lava mx-auto mb-4 flex h-12 w-12 items-center
                        justify-center rounded-xl"
                >
                    <div class="bg-lava animate-pulse-glow h-3 w-3 rounded-full"></div>
                </div>
                <h1 class="text-xl font-bold tracking-tight text-white">
                    GOON<span class="text-lava">HUB</span>
                </h1>
                <p class="text-dim mt-1.5 text-xs">Sign in to access your library</p>
            </div>

            <!-- Login Card -->
            <div class="glass-panel p-6">
                <form @submit.prevent="handleLogin" class="space-y-4">
                    <div>
                        <label
                            for="username"
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Username
                        </label>
                        <input
                            id="username"
                            v-model="username"
                            type="text"
                            :disabled="isLoading"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none
                                disabled:opacity-50"
                            placeholder="Enter username"
                            autocomplete="username"
                        />
                    </div>

                    <div>
                        <label
                            for="password"
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Password
                        </label>
                        <input
                            id="password"
                            v-model="password"
                            type="password"
                            :disabled="isLoading"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none
                                disabled:opacity-50"
                            placeholder="Enter password"
                            autocomplete="current-password"
                        />
                    </div>

                    <div
                        v-if="error"
                        class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2
                            text-xs"
                    >
                        {{ error }}
                    </div>

                    <button
                        type="submit"
                        :disabled="isLoading || !username || !password"
                        class="bg-lava hover:bg-lava-glow glow-lava w-full rounded-lg px-4 py-2.5
                            text-sm font-semibold text-white transition-all
                            disabled:cursor-not-allowed disabled:opacity-40 disabled:shadow-none"
                    >
                        <span v-if="isLoading" class="flex items-center justify-center gap-2">
                            <div
                                class="h-3 w-3 animate-spin rounded-full border-2 border-white/30
                                    border-t-white"
                            ></div>
                            Signing in...
                        </span>
                        <span v-else>Sign In</span>
                    </button>
                </form>
            </div>
        </div>
    </div>
</template>
