<script setup lang="ts">
import type { RoleResponse } from '~/types/admin';

const props = defineProps<{
    visible: boolean;
    roles: RoleResponse[];
}>();

const emit = defineEmits<{
    close: [];
    created: [];
}>();

const { createUser } = useApi();

const username = ref('');
const password = ref('');
const role = ref('user');
const loading = ref(false);
const error = ref('');

const handleSubmit = async () => {
    error.value = '';
    loading.value = true;
    try {
        await createUser(username.value, password.value, role.value);
        username.value = '';
        password.value = '';
        role.value = 'user';
        emit('created');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to create user';
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    error.value = '';
    emit('close');
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div class="glass-panel border-border w-full max-w-sm border p-6">
                <h3 class="mb-4 text-sm font-semibold text-white">Create User</h3>
                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>
                <form @submit.prevent="handleSubmit" class="space-y-3">
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Username
                        </label>
                        <input
                            v-model="username"
                            type="text"
                            autocomplete="username"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none"
                            placeholder="Username"
                        />
                    </div>
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Password
                        </label>
                        <input
                            v-model="password"
                            type="password"
                            autocomplete="new-password"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none"
                            placeholder="Password (min 6 chars)"
                        />
                    </div>
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Role
                        </label>
                        <UiSelectMenu
                            v-model="role"
                            :options="roles.map((r) => ({ value: r.name, label: r.name }))"
                        />
                    </div>
                    <div class="flex justify-end gap-2 pt-2">
                        <button
                            type="button"
                            @click="handleClose"
                            class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                                hover:text-white"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            :disabled="loading || !username || !password"
                            class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                font-semibold text-white transition-all disabled:cursor-not-allowed
                                disabled:opacity-40"
                        >
                            Create
                        </button>
                    </div>
                </form>
            </div>
        </div>
    </Teleport>
</template>
