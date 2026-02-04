<script setup lang="ts">
import type { StoragePath, ValidatePathResponse } from '~/types/storage';

const props = defineProps<{
    visible: boolean;
    storagePath: StoragePath | null;
}>();

const emit = defineEmits<{
    close: [];
    saved: [];
}>();

const { createStoragePath, updateStoragePath, validateStoragePath } = useApi();

const name = ref('');
const path = ref('');
const isDefault = ref(false);
const loading = ref(false);
const validating = ref(false);
const error = ref('');
const validation = ref<ValidatePathResponse | null>(null);

const isEdit = computed(() => props.storagePath !== null);

watch(
    () => props.visible,
    (visible) => {
        if (visible) {
            if (props.storagePath) {
                name.value = props.storagePath.name;
                path.value = props.storagePath.path;
                isDefault.value = props.storagePath.is_default;
            } else {
                name.value = '';
                path.value = '';
                isDefault.value = false;
            }
            error.value = '';
            validation.value = null;
        }
    },
);

const handleValidate = async () => {
    if (!path.value) return;

    validating.value = true;
    validation.value = null;
    try {
        validation.value = await validateStoragePath(path.value);
    } catch (e: unknown) {
        validation.value = {
            valid: false,
            message: e instanceof Error ? e.message : 'Validation failed',
        };
    } finally {
        validating.value = false;
    }
};

const handleSubmit = async () => {
    error.value = '';
    loading.value = true;
    try {
        if (isEdit.value && props.storagePath) {
            await updateStoragePath(props.storagePath.id, name.value, path.value, isDefault.value);
        } else {
            await createStoragePath(name.value, path.value, isDefault.value);
        }
        emit('saved');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save storage path';
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    error.value = '';
    validation.value = null;
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
            <div class="glass-panel border-border w-full max-w-md border p-6">
                <h3 class="mb-4 text-sm font-semibold text-white">
                    {{ isEdit ? 'Edit Storage Path' : 'Add Storage Path' }}
                </h3>
                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>
                <form class="space-y-4" @submit.prevent="handleSubmit">
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Name
                        </label>
                        <input
                            v-model="name"
                            type="text"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3.5 py-2.5 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none"
                            placeholder="e.g., External Drive, NAS Movies"
                        />
                    </div>
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Path
                        </label>
                        <div class="flex gap-2">
                            <input
                                v-model="path"
                                type="text"
                                class="border-border bg-void/80 placeholder-dim/50
                                    focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg border
                                    px-3.5 py-2.5 font-mono text-sm text-white transition-all
                                    focus:ring-1 focus:outline-none"
                                placeholder="/app/external/movies"
                            />
                            <button
                                type="button"
                                :disabled="validating || !path"
                                class="border-border rounded-lg border px-3 text-xs text-white
                                    transition-all hover:border-white/20 hover:bg-white/5
                                    disabled:cursor-not-allowed disabled:opacity-40"
                                @click="handleValidate"
                            >
                                {{ validating ? '...' : 'Validate' }}
                            </button>
                        </div>
                        <div
                            v-if="validation"
                            class="mt-2 rounded-lg px-3 py-2 text-xs"
                            :class="
                                validation.valid
                                    ? 'border-emerald/20 bg-emerald/5 text-emerald border'
                                    : 'border-lava/20 bg-lava/5 text-lava border'
                            "
                        >
                            {{ validation.message }}
                        </div>
                    </div>
                    <div>
                        <label class="flex cursor-pointer items-center gap-2">
                            <input
                                v-model="isDefault"
                                type="checkbox"
                                class="accent-lava h-4 w-4 rounded"
                            />
                            <span class="text-xs text-white">Set as default storage path</span>
                        </label>
                        <p class="text-dim mt-1 pl-6 text-[11px]">
                            New uploads will be stored in the default path
                        </p>
                    </div>
                    <div class="flex justify-end gap-2 pt-2">
                        <button
                            type="button"
                            class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                                hover:text-white"
                            @click="handleClose"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            :disabled="loading || !name || !path"
                            class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                font-semibold text-white transition-all disabled:cursor-not-allowed
                                disabled:opacity-40"
                        >
                            {{ loading ? 'Saving...' : isEdit ? 'Update' : 'Create' }}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    </Teleport>
</template>
