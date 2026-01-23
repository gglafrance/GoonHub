<script setup lang="ts">
const { changePassword, changeUsername } = useApi();
const authStore = useAuthStore();
const { message, error, clearMessages } = useSettingsMessage();

const newUsername = ref('');
const currentPassword = ref('');
const newPassword = ref('');
const confirmPassword = ref('');
const accountLoading = ref(false);

const handleChangeUsername = async () => {
    clearMessages();
    if (!newUsername.value || newUsername.value.length < 3) {
        error.value = 'Username must be at least 3 characters';
        return;
    }
    accountLoading.value = true;
    try {
        await changeUsername(newUsername.value);
        message.value = 'Username changed successfully';
        authStore.user!.username = newUsername.value;
        newUsername.value = '';
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to change username';
    } finally {
        accountLoading.value = false;
    }
};

const handleChangePassword = async () => {
    clearMessages();
    if (!currentPassword.value || !newPassword.value) {
        error.value = 'All password fields are required';
        return;
    }
    if (newPassword.value !== confirmPassword.value) {
        error.value = 'New passwords do not match';
        return;
    }
    if (newPassword.value.length < 6) {
        error.value = 'New password must be at least 6 characters';
        return;
    }
    accountLoading.value = true;
    try {
        await changePassword(currentPassword.value, newPassword.value);
        message.value = 'Password changed successfully';
        currentPassword.value = '';
        newPassword.value = '';
        confirmPassword.value = '';
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to change password';
    } finally {
        accountLoading.value = false;
    }
};
</script>

<template>
    <div class="space-y-6">
        <div
            v-if="message"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2 text-xs"
        >
            {{ message }}
        </div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <!-- Username Change -->
        <div class="glass-panel p-5">
            <h3 class="mb-4 text-sm font-semibold text-white">Change Username</h3>
            <form @submit.prevent="handleChangeUsername" class="space-y-3">
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
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
                        text-white transition-all disabled:cursor-not-allowed disabled:opacity-40"
                >
                    Update Username
                </button>
            </form>
        </div>

        <!-- Password Change -->
        <div class="glass-panel p-5">
            <h3 class="mb-4 text-sm font-semibold text-white">Change Password</h3>
            <form @submit.prevent="handleChangePassword" class="space-y-3">
                <input type="text" :value="authStore.user?.username" autocomplete="username" class="hidden" aria-hidden="true" tabindex="-1" />
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
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
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
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
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
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
                    :disabled="accountLoading || !currentPassword || !newPassword || !confirmPassword"
                    class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                        text-white transition-all disabled:cursor-not-allowed disabled:opacity-40"
                >
                    Update Password
                </button>
            </form>
        </div>
    </div>
</template>
