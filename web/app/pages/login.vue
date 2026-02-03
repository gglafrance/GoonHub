<script setup lang="ts">
const authStore = useAuthStore();

useHead({ title: 'Login' });

useSeoMeta({
    title: 'Login',
    ogTitle: 'Login - GoonHub',
    description: 'Sign in to access your scene library',
    ogDescription: 'Sign in to access your scene library',
});

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
    <div
        class="pt-safe-top flex min-h-[calc(100svh-3.1rem)] flex-col justify-between px-5 pb-8
            sm:min-h-[calc(100vh-3.1rem)] sm:flex-row sm:items-center sm:justify-center sm:px-4
            sm:pt-0 sm:pb-0"
    >
        <!-- Mobile: Ambient glow effect at top -->
        <div
            class="from-lava/3 pointer-events-none fixed inset-x-0 top-0 h-64 bg-linear-to-b
                to-transparent sm:hidden"
            aria-hidden="true"
        />

        <div class="relative z-10 w-full max-w-sm sm:mx-auto">
            <!-- Logo Section - More prominent on mobile -->
            <div class="mb-10 pt-12 text-center sm:mb-8 sm:pt-0">
                <div
                    class="bg-lava/10 glow-lava mx-auto mb-5 flex h-16 w-16 items-center
                        justify-center rounded-2xl sm:mb-4 sm:h-12 sm:w-12 sm:rounded-xl"
                >
                    <div
                        class="bg-lava animate-pulse-glow h-4 w-4 rounded-full sm:h-3 sm:w-3"
                    ></div>
                </div>
                <h1 class="text-2xl font-bold tracking-tight text-white sm:text-xl">
                    GOON<span class="text-lava">HUB</span>
                </h1>
                <p class="text-dim mt-2 text-sm sm:mt-1.5 sm:text-xs">
                    Sign in to access your library
                </p>
            </div>

            <!-- Login Card -->
            <div class="glass-panel p-5 sm:p-6">
                <form @submit.prevent="handleLogin" class="space-y-5 sm:space-y-4">
                    <div>
                        <label
                            for="username"
                            class="text-dim mb-2 block text-xs font-medium tracking-wider uppercase
                                sm:mb-1.5 sm:text-[11px]"
                        >
                            Username
                        </label>
                        <input
                            id="username"
                            v-model="username"
                            type="text"
                            :disabled="isLoading"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/50
                                focus:ring-lava/25 sm:focus:border-lava/40 sm:focus:ring-lava/20
                                w-full rounded-xl border px-4 py-3.5 text-base text-white
                                transition-all duration-200 focus:ring-2 focus:outline-none
                                disabled:opacity-50 sm:rounded-lg sm:px-3.5 sm:py-2.5 sm:text-sm
                                sm:focus:ring-1"
                            placeholder="Enter username"
                            autocomplete="username"
                            inputmode="text"
                            enterkeyhint="next"
                        />
                    </div>

                    <div>
                        <label
                            for="password"
                            class="text-dim mb-2 block text-xs font-medium tracking-wider uppercase
                                sm:mb-1.5 sm:text-[11px]"
                        >
                            Password
                        </label>
                        <input
                            id="password"
                            v-model="password"
                            type="password"
                            :disabled="isLoading"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/50
                                focus:ring-lava/25 sm:focus:border-lava/40 sm:focus:ring-lava/20
                                w-full rounded-xl border px-4 py-3.5 text-base text-white
                                transition-all duration-200 focus:ring-2 focus:outline-none
                                disabled:opacity-50 sm:rounded-lg sm:px-3.5 sm:py-2.5 sm:text-sm
                                sm:focus:ring-1"
                            placeholder="Enter password"
                            autocomplete="current-password"
                            enterkeyhint="go"
                        />
                    </div>

                    <!-- Error message -->
                    <div
                        v-if="error"
                        class="border-lava/20 bg-lava/5 text-lava flex items-start gap-2.5
                            rounded-xl border px-4 py-3 text-sm sm:rounded-lg sm:px-3 sm:py-2
                            sm:text-xs"
                    >
                        <Icon
                            name="heroicons:exclamation-triangle-16-solid"
                            class="mt-0.5 h-4 w-4 shrink-0 sm:hidden"
                        />
                        <span>{{ error }}</span>
                    </div>

                    <button
                        type="submit"
                        :disabled="isLoading || !username || !password"
                        class="bg-lava hover:bg-lava-glow glow-lava mt-2 w-full rounded-xl px-4 py-4
                            text-base font-semibold text-white transition-all duration-200
                            active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-40
                            disabled:shadow-none disabled:active:scale-100 sm:mt-0 sm:rounded-lg
                            sm:py-2.5 sm:text-sm sm:active:scale-100"
                    >
                        <span v-if="isLoading" class="flex items-center justify-center gap-2.5">
                            <div
                                class="h-4 w-4 animate-spin rounded-full border-2 border-white/30
                                    border-t-white sm:h-3 sm:w-3"
                            ></div>
                            Signing in...
                        </span>
                        <span v-else>Sign In</span>
                    </button>
                </form>
            </div>

            <!-- Mobile: Bottom safe area spacer -->
            <div class="h-safe-bottom sm:hidden" />
        </div>
    </div>
</template>
