<script setup lang="ts">
import { useAuth } from '~/composables/useAuth';
import { useAuthStore } from '~/stores/auth';

const authComposable = useAuth();
const authStore = useAuthStore();

const username = ref('');
const password = ref('');
const isLoading = ref(false);
const error = ref('');

const route = useRoute();

const handleLogin = async () => {
    isLoading.value = true;
    error.value = '';

    try {
        authStore.setLoading(true);
        authStore.clearError();

        const data = await authComposable.login(username.value, password.value);

        const redirect = route.query.redirect as string;
        navigateTo(redirect || '/');
    } catch (e: any) {
        error.value = e.message || 'Invalid credentials';
    } finally {
        isLoading.value = false;
        authStore.setLoading(false);
    }
};

definePageMeta({
    title: 'Login - GoonHub',
});
</script>

<template>
    <div class="bg-primary flex min-h-screen items-center justify-center px-4 py-12">
        <div
            class="bg-secondary/30 w-full max-w-md rounded-2xl border border-white/5 p-8
                backdrop-blur-md"
        >
            <div class="mb-8 text-center">
                <h1 class="text-4xl font-extrabold tracking-tight text-white">
                    Goon<span class="text-neon-green">Hub</span>
                </h1>
                <p class="mt-2 text-sm text-gray-400">Sign in to access your library</p>
            </div>

            <form @submit.prevent="handleLogin" class="space-y-6">
                <div>
                    <label for="username" class="mb-2 block text-sm font-medium text-gray-300">
                        Username
                    </label>
                    <input
                        id="username"
                        v-model="username"
                        type="text"
                        :disabled="isLoading"
                        class="focus:border-neon-green/50 focus:ring-neon-green/50 w-full rounded-xl
                            border border-white/10 bg-black/50 px-4 py-3 text-white transition-all
                            focus:ring-1 focus:outline-none disabled:opacity-50"
                        placeholder="Enter your username"
                        autocomplete="username"
                    />
                </div>

                <div>
                    <label for="password" class="mb-2 block text-sm font-medium text-gray-300">
                        Password
                    </label>
                    <input
                        id="password"
                        v-model="password"
                        type="password"
                        :disabled="isLoading"
                        class="focus:border-neon-green/50 focus:ring-neon-green/50 w-full rounded-xl
                            border border-white/10 bg-black/50 px-4 py-3 text-white transition-all
                            focus:ring-1 focus:outline-none disabled:opacity-50"
                        placeholder="Enter your password"
                        autocomplete="current-password"
                    />
                </div>

                <div
                    v-if="error"
                    class="bg-neon-red/10 border-neon-red/20 text-neon-red rounded-xl border p-3
                        text-sm"
                >
                    {{ error }}
                </div>

                <button
                    type="submit"
                    :disabled="isLoading || !username || !password"
                    class="bg-neon-green hover:bg-neon-green/90 w-full rounded-xl px-4 py-3
                        font-bold text-black transition-all duration-300 disabled:cursor-not-allowed
                        disabled:opacity-50"
                >
                    <span v-if="isLoading">Signing in...</span>
                    <span v-else>Sign In</span>
                </button>
            </form>
        </div>
    </div>
</template>
