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
const showPassword = ref(false);
const mounted = ref(false);

const route = useRoute();
const usernameInput = ref<HTMLInputElement>();

onMounted(() => {
    requestAnimationFrame(() => {
        mounted.value = true;
    });
    nextTick(() => {
        usernameInput.value?.focus();
    });
});

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
        class="relative flex min-h-[calc(100svh-3.1rem)] items-center justify-center px-5 py-12
            sm:min-h-[calc(100vh-3.1rem)] sm:px-4"
    >
        <!-- Ambient background effects -->
        <div class="pointer-events-none fixed inset-0 z-0" aria-hidden="true">
            <div class="login-orb-primary" />
            <div class="login-orb-secondary" />
            <div class="login-noise" />
        </div>

        <!-- Login container -->
        <div
            class="relative z-10 mx-auto w-full max-w-90"
            :class="mounted ? 'login-enter-active' : 'login-enter'"
        >
            <!-- Logo -->
            <div class="mb-8 text-center sm:mb-7">
                <div
                    class="login-logo-glow bg-lava/8 border-lava/12 mx-auto mb-4 flex h-12 w-12
                        items-center justify-center rounded-[14px] border"
                >
                    <div class="login-dot-breathe bg-lava h-2.5 w-2.5 rounded-full" />
                </div>
                <h1 class="text-[22px] font-bold tracking-tight text-white sm:text-xl">
                    GOON<span class="text-lava">HUB</span>
                </h1>
                <p class="text-dim mt-1.5 text-[13px] sm:text-xs">Sign in to your library</p>
            </div>

            <!-- Login Card -->
            <div class="login-card">
                <form class="space-y-4" @submit.prevent="handleLogin">
                    <!-- Username -->
                    <div class="flex flex-col gap-1.5">
                        <label
                            for="username"
                            class="text-dim flex items-center gap-1.5 text-[11px] font-medium
                                tracking-wider uppercase"
                        >
                            <Icon name="heroicons:user-16-solid" class="h-3 w-3 opacity-50" />
                            Username
                        </label>
                        <input
                            id="username"
                            ref="usernameInput"
                            v-model="username"
                            type="text"
                            :disabled="isLoading"
                            class="login-input bg-void/60 w-full rounded-[10px] border
                                border-white/7 px-3.5 py-2.75 text-sm text-white transition-all
                                duration-200 outline-none placeholder:text-white/20
                                disabled:cursor-not-allowed disabled:opacity-50 max-sm:rounded-xl
                                max-sm:py-3.25 max-sm:text-base"
                            placeholder="Enter username"
                            autocomplete="username"
                            inputmode="text"
                            enterkeyhint="next"
                        />
                    </div>

                    <!-- Password -->
                    <div class="flex flex-col gap-1.5">
                        <label
                            for="password"
                            class="text-dim flex items-center gap-1.5 text-[11px] font-medium
                                tracking-wider uppercase"
                        >
                            <Icon
                                name="heroicons:lock-closed-16-solid"
                                class="h-3 w-3 opacity-50"
                            />
                            Password
                        </label>
                        <div class="relative">
                            <input
                                id="password"
                                v-model="password"
                                :type="showPassword ? 'text' : 'password'"
                                :disabled="isLoading"
                                class="login-input bg-void/60 w-full rounded-[10px] border
                                    border-white/7 px-3.5 py-2.75 pr-10 text-sm text-white
                                    transition-all duration-200 outline-none
                                    placeholder:text-white/20 disabled:cursor-not-allowed
                                    disabled:opacity-50 max-sm:rounded-xl max-sm:py-3.25
                                    max-sm:text-base"
                                placeholder="Enter password"
                                autocomplete="current-password"
                                enterkeyhint="go"
                            />
                            <button
                                type="button"
                                class="text-dim hover:text-muted absolute top-1/2 right-3 flex
                                    -translate-y-1/2 items-center justify-center transition-colors"
                                tabindex="-1"
                                @click="showPassword = !showPassword"
                            >
                                <Icon
                                    :name="
                                        showPassword
                                            ? 'heroicons:eye-slash-16-solid'
                                            : 'heroicons:eye-16-solid'
                                    "
                                    class="h-4 w-4"
                                />
                            </button>
                        </div>
                    </div>

                    <!-- Error -->
                    <div
                        v-if="error"
                        class="login-error text-lava bg-lava/6 border-lava/12 flex items-start gap-2
                            rounded-[10px] border px-3 py-2.5 text-xs"
                    >
                        <Icon
                            name="heroicons:exclamation-triangle-16-solid"
                            class="mt-px h-3.5 w-3.5 shrink-0"
                        />
                        <span>{{ error }}</span>
                    </div>

                    <!-- Submit -->
                    <button
                        type="submit"
                        :disabled="isLoading || !username || !password"
                        class="login-submit bg-lava hover:bg-lava-glow mt-1 w-full cursor-pointer
                            rounded-[10px] border-none px-4 py-3 text-sm font-semibold text-white
                            transition-all duration-200 disabled:cursor-not-allowed
                            disabled:opacity-35 max-sm:rounded-xl max-sm:py-3.5 max-sm:text-[15px]"
                    >
                        <span v-if="isLoading" class="flex items-center justify-center gap-2">
                            <div
                                class="h-3.5 w-3.5 animate-spin rounded-full border-2
                                    border-white/20 border-t-white"
                            />
                            Signing in...
                        </span>
                        <span v-else class="flex items-center justify-center gap-2">
                            Sign In
                            <Icon name="heroicons:arrow-right-16-solid" class="h-4 w-4" />
                        </span>
                    </button>
                </form>
            </div>
        </div>
    </div>
</template>

<style scoped>
/* ── Ambient orbs (radial gradients + blur need custom CSS) ── */
.login-orb-primary,
.login-orb-secondary {
    position: absolute;
    border-radius: 50%;
    filter: blur(100px);
}

.login-orb-primary {
    width: 500px;
    height: 500px;
    top: -200px;
    left: 50%;
    transform: translateX(-50%);
    background: radial-gradient(circle, rgba(255, 77, 77, 0.08) 0%, transparent 70%);
    animation: orb-drift 12s ease-in-out infinite;
}

.login-orb-secondary {
    width: 400px;
    height: 400px;
    bottom: -150px;
    right: -100px;
    background: radial-gradient(circle, rgba(255, 77, 77, 0.04) 0%, transparent 70%);
    animation: orb-drift 15s ease-in-out infinite reverse;
}

.login-noise {
    position: absolute;
    inset: 0;
    opacity: 0.015;
    background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)'/%3E%3C/svg%3E");
    background-repeat: repeat;
    background-size: 256px 256px;
}

@keyframes orb-drift {
    0%,
    100% {
        transform: translateX(-50%) translateY(0);
    }
    50% {
        transform: translateX(-50%) translateY(20px);
    }
}

/* ── Entrance animation ── */
.login-enter {
    opacity: 0;
    transform: translateY(12px);
}

.login-enter-active {
    opacity: 1;
    transform: translateY(0);
    transition:
        opacity 0.5s cubic-bezier(0.16, 1, 0.3, 1),
        transform 0.5s cubic-bezier(0.16, 1, 0.3, 1);
}

/* ── Logo glow + breathing dot ── */
.login-logo-glow {
    box-shadow:
        0 0 30px rgba(255, 77, 77, 0.08),
        0 0 60px rgba(255, 77, 77, 0.04);
}

.login-dot-breathe {
    animation: dot-breathe 3s ease-in-out infinite;
}

@keyframes dot-breathe {
    0%,
    100% {
        opacity: 0.6;
        box-shadow: 0 0 12px rgba(255, 77, 77, 0.4);
    }
    50% {
        opacity: 1;
        box-shadow: 0 0 20px rgba(255, 77, 77, 0.6);
    }
}

/* ── Card (gradient border pseudo-element) ── */
.login-card {
    background: rgba(10, 10, 10, 0.5);
    backdrop-filter: blur(20px);
    -webkit-backdrop-filter: blur(20px);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 16px;
    padding: 24px;
    position: relative;
}

.login-card::before {
    content: '';
    position: absolute;
    inset: -1px;
    border-radius: 17px;
    padding: 1px;
    background: linear-gradient(
        180deg,
        rgba(255, 255, 255, 0.06) 0%,
        rgba(255, 255, 255, 0.02) 50%,
        transparent 100%
    );
    mask:
        linear-gradient(#000 0 0) content-box,
        linear-gradient(#000 0 0);
    mask-composite: exclude;
    pointer-events: none;
}

@media (max-width: 639px) {
    .login-card {
        padding: 20px;
        border-radius: 14px;
    }
    .login-card::before {
        border-radius: 15px;
    }
}

/* ── Input focus glow ── */
.login-input:focus {
    border-color: rgba(255, 77, 77, 0.35);
    box-shadow:
        0 0 0 3px rgba(255, 77, 77, 0.08),
        0 0 20px rgba(255, 77, 77, 0.05);
}

/* ── Error shake ── */
.login-error {
    animation: error-shake 0.4s cubic-bezier(0.36, 0.07, 0.19, 0.97);
}

@keyframes error-shake {
    0%,
    100% {
        transform: translateX(0);
    }
    20% {
        transform: translateX(-6px);
    }
    40% {
        transform: translateX(5px);
    }
    60% {
        transform: translateX(-3px);
    }
    80% {
        transform: translateX(2px);
    }
}

/* ── Submit glow ── */
.login-submit {
    box-shadow:
        0 0 20px rgba(255, 77, 77, 0.2),
        0 2px 8px rgba(0, 0, 0, 0.3);
}

.login-submit:hover:not(:disabled) {
    box-shadow:
        0 0 30px rgba(255, 77, 77, 0.3),
        0 4px 12px rgba(0, 0, 0, 0.3);
    transform: translateY(-1px);
}

.login-submit:active:not(:disabled) {
    transform: translateY(0) scale(0.99);
    box-shadow:
        0 0 15px rgba(255, 77, 77, 0.15),
        0 1px 4px rgba(0, 0, 0, 0.3);
}

.login-submit:disabled {
    box-shadow: none;
}
</style>
