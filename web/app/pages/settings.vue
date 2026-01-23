<script setup lang="ts">
import type { SortOrder } from '~/types/settings';

const settingsStore = useSettingsStore();
const authStore = useAuthStore();
const { changePassword, changeUsername } = useApi();

const activeTab = ref<'account' | 'player' | 'app'>('account');

// Account tab state
const newUsername = ref('');
const currentPassword = ref('');
const newPassword = ref('');
const confirmPassword = ref('');
const accountMessage = ref('');
const accountError = ref('');
const accountLoading = ref(false);

// Player tab state
const playerAutoplay = ref(false);
const playerVolume = ref(100);
const playerLoop = ref(false);
const playerMessage = ref('');
const playerError = ref('');

// App tab state
const appVideosPerPage = ref(20);
const appSortOrder = ref<SortOrder>('created_at_desc');
const appMessage = ref('');
const appError = ref('');

const sortOptions: { value: SortOrder; label: string }[] = [
    { value: 'created_at_desc', label: 'Newest First' },
    { value: 'created_at_asc', label: 'Oldest First' },
    { value: 'title_asc', label: 'Title A-Z' },
    { value: 'title_desc', label: 'Title Z-A' },
    { value: 'duration_asc', label: 'Shortest First' },
    { value: 'duration_desc', label: 'Longest First' },
    { value: 'size_asc', label: 'Smallest First' },
    { value: 'size_desc', label: 'Largest First' },
];

// Initialize form values from store
const initFormValues = () => {
    playerAutoplay.value = settingsStore.autoplay;
    playerVolume.value = settingsStore.defaultVolume;
    playerLoop.value = settingsStore.loop;
    appVideosPerPage.value = settingsStore.videosPerPage;
    appSortOrder.value = settingsStore.defaultSortOrder;
};

onMounted(async () => {
    await settingsStore.loadSettings();
    initFormValues();
});

watch(
    () => settingsStore.settings,
    () => {
        initFormValues();
    },
);

const handleChangeUsername = async () => {
    accountMessage.value = '';
    accountError.value = '';
    if (!newUsername.value || newUsername.value.length < 3) {
        accountError.value = 'Username must be at least 3 characters';
        return;
    }
    accountLoading.value = true;
    try {
        await changeUsername(newUsername.value);
        accountMessage.value = 'Username changed successfully';
        authStore.user!.username = newUsername.value;
        newUsername.value = '';
    } catch (e: unknown) {
        accountError.value = e instanceof Error ? e.message : 'Failed to change username';
    } finally {
        accountLoading.value = false;
    }
};

const handleChangePassword = async () => {
    accountMessage.value = '';
    accountError.value = '';
    if (!currentPassword.value || !newPassword.value) {
        accountError.value = 'All password fields are required';
        return;
    }
    if (newPassword.value !== confirmPassword.value) {
        accountError.value = 'New passwords do not match';
        return;
    }
    if (newPassword.value.length < 6) {
        accountError.value = 'New password must be at least 6 characters';
        return;
    }
    accountLoading.value = true;
    try {
        await changePassword(currentPassword.value, newPassword.value);
        accountMessage.value = 'Password changed successfully';
        currentPassword.value = '';
        newPassword.value = '';
        confirmPassword.value = '';
    } catch (e: unknown) {
        accountError.value = e instanceof Error ? e.message : 'Failed to change password';
    } finally {
        accountLoading.value = false;
    }
};

const handleSavePlayer = async () => {
    playerMessage.value = '';
    playerError.value = '';
    try {
        await settingsStore.updatePlayer(
            playerAutoplay.value,
            playerVolume.value,
            playerLoop.value,
        );
        playerMessage.value = 'Player settings saved';
    } catch (e: unknown) {
        playerError.value = e instanceof Error ? e.message : 'Failed to save settings';
    }
};

const handleSaveApp = async () => {
    appMessage.value = '';
    appError.value = '';
    try {
        await settingsStore.updateApp(appVideosPerPage.value, appSortOrder.value);
        appMessage.value = 'App settings saved';
    } catch (e: unknown) {
        appError.value = e instanceof Error ? e.message : 'Failed to save settings';
    }
};

definePageMeta({
    title: 'Settings - GoonHub',
});
</script>

<template>
    <div class="mx-auto max-w-2xl px-4 py-8 sm:px-5">
        <h1 class="mb-6 text-lg font-bold tracking-tight text-white">Settings</h1>

        <!-- Tabs -->
        <div class="border-border mb-6 flex gap-1 border-b pb-px">
            <button
                v-for="tab in ['account', 'player', 'app'] as const"
                :key="tab"
                @click="activeTab = tab"
                class="relative px-4 py-2 text-xs font-medium capitalize transition-colors"
                :class="activeTab === tab ? 'text-lava' : 'text-dim hover:text-white'"
            >
                {{ tab }}
                <div
                    v-if="activeTab === tab"
                    class="bg-lava absolute right-0 bottom-0 left-0 h-0.5 rounded-full"
                ></div>
            </button>
        </div>

        <!-- Account Tab -->
        <div v-if="activeTab === 'account'" class="space-y-6">
            <!-- Success/Error Messages -->
            <div
                v-if="accountMessage"
                class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2
                    text-xs"
            >
                {{ accountMessage }}
            </div>
            <div
                v-if="accountError"
                class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
            >
                {{ accountError }}
            </div>

            <!-- Username Change -->
            <div class="glass-panel p-5">
                <h3 class="mb-4 text-sm font-semibold text-white">Change Username</h3>
                <form @submit.prevent="handleChangeUsername" class="space-y-3">
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            New Username
                        </label>
                        <input
                            v-model="newUsername"
                            type="text"
                            :disabled="accountLoading"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none
                                disabled:opacity-50"
                            placeholder="Enter new username"
                        />
                    </div>
                    <button
                        type="submit"
                        :disabled="accountLoading || !newUsername"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                            text-white transition-all disabled:cursor-not-allowed
                            disabled:opacity-40"
                    >
                        Update Username
                    </button>
                </form>
            </div>

            <!-- Password Change -->
            <div class="glass-panel p-5">
                <h3 class="mb-4 text-sm font-semibold text-white">Change Password</h3>
                <form @submit.prevent="handleChangePassword" class="space-y-3">
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Current Password
                        </label>
                        <input
                            v-model="currentPassword"
                            type="password"
                            :disabled="accountLoading"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none
                                disabled:opacity-50"
                            placeholder="Enter current password"
                            autocomplete="current-password"
                        />
                    </div>
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            New Password
                        </label>
                        <input
                            v-model="newPassword"
                            type="password"
                            :disabled="accountLoading"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none
                                disabled:opacity-50"
                            placeholder="Enter new password"
                            autocomplete="new-password"
                        />
                    </div>
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Confirm New Password
                        </label>
                        <input
                            v-model="confirmPassword"
                            type="password"
                            :disabled="accountLoading"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none
                                disabled:opacity-50"
                            placeholder="Confirm new password"
                            autocomplete="new-password"
                        />
                    </div>
                    <button
                        type="submit"
                        :disabled="
                            accountLoading || !currentPassword || !newPassword || !confirmPassword
                        "
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                            text-white transition-all disabled:cursor-not-allowed
                            disabled:opacity-40"
                    >
                        Update Password
                    </button>
                </form>
            </div>
        </div>

        <!-- Player Tab -->
        <div v-if="activeTab === 'player'" class="space-y-6">
            <div
                v-if="playerMessage"
                class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2
                    text-xs"
            >
                {{ playerMessage }}
            </div>
            <div
                v-if="playerError"
                class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
            >
                {{ playerError }}
            </div>

            <div class="glass-panel p-5">
                <h3 class="mb-5 text-sm font-semibold text-white">Player Preferences</h3>
                <div class="space-y-5">
                    <!-- Autoplay Toggle -->
                    <div class="flex items-center justify-between">
                        <div>
                            <div class="text-sm text-white">Autoplay</div>
                            <div class="text-dim text-xs">
                                Automatically play videos when opened
                            </div>
                        </div>
                        <button
                            @click="playerAutoplay = !playerAutoplay"
                            class="relative h-5 w-9 rounded-full transition-colors"
                            :class="playerAutoplay ? 'bg-emerald' : 'bg-border'"
                        >
                            <div
                                class="absolute top-0.5 h-4 w-4 rounded-full bg-white shadow
                                    transition-transform"
                                :class="playerAutoplay ? 'translate-x-4' : 'translate-x-0.5'"
                            ></div>
                        </button>
                    </div>

                    <!-- Loop Toggle -->
                    <div class="flex items-center justify-between">
                        <div>
                            <div class="text-sm text-white">Loop</div>
                            <div class="text-dim text-xs">Loop videos when they finish</div>
                        </div>
                        <button
                            @click="playerLoop = !playerLoop"
                            class="relative h-5 w-9 rounded-full transition-colors"
                            :class="playerLoop ? 'bg-emerald' : 'bg-border'"
                        >
                            <div
                                class="absolute top-0.5 h-4 w-4 rounded-full bg-white shadow
                                    transition-transform"
                                :class="playerLoop ? 'translate-x-4' : 'translate-x-0.5'"
                            ></div>
                        </button>
                    </div>

                    <!-- Volume Slider -->
                    <div>
                        <div class="mb-2 flex items-center justify-between">
                            <div>
                                <div class="text-sm text-white">Default Volume</div>
                                <div class="text-dim text-xs">Initial volume level for videos</div>
                            </div>
                            <span class="text-dim font-mono text-xs">{{ playerVolume }}%</span>
                        </div>
                        <input
                            v-model.number="playerVolume"
                            type="range"
                            min="0"
                            max="100"
                            step="5"
                            class="accent-lava w-full"
                        />
                    </div>

                    <button
                        @click="handleSavePlayer"
                        :disabled="settingsStore.isLoading"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                            text-white transition-all disabled:cursor-not-allowed
                            disabled:opacity-40"
                    >
                        Save Player Settings
                    </button>
                </div>
            </div>
        </div>

        <!-- App Tab -->
        <div v-if="activeTab === 'app'" class="space-y-6">
            <div
                v-if="appMessage"
                class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2
                    text-xs"
            >
                {{ appMessage }}
            </div>
            <div
                v-if="appError"
                class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
            >
                {{ appError }}
            </div>

            <div class="glass-panel p-5">
                <h3 class="mb-5 text-sm font-semibold text-white">App Preferences</h3>
                <div class="space-y-5">
                    <!-- Videos Per Page -->
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Videos Per Page
                        </label>
                        <input
                            v-model.number="appVideosPerPage"
                            type="number"
                            min="1"
                            max="100"
                            class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                                w-full max-w-32 rounded-lg border px-3.5 py-2.5 text-sm text-white
                                transition-all focus:ring-1 focus:outline-none"
                        />
                    </div>

                    <!-- Sort Order -->
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Default Sort Order
                        </label>
                        <select
                            v-model="appSortOrder"
                            class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                                w-full max-w-64 appearance-none rounded-lg border px-3.5 py-2.5
                                text-sm text-white transition-all focus:ring-1 focus:outline-none"
                        >
                            <option
                                v-for="opt in sortOptions"
                                :key="opt.value"
                                :value="opt.value"
                                class="bg-panel"
                            >
                                {{ opt.label }}
                            </option>
                        </select>
                    </div>

                    <button
                        @click="handleSaveApp"
                        :disabled="settingsStore.isLoading"
                        class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                            text-white transition-all disabled:cursor-not-allowed
                            disabled:opacity-40"
                    >
                        Save App Settings
                    </button>
                </div>
            </div>
        </div>
    </div>
</template>
