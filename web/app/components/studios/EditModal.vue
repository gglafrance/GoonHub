<script setup lang="ts">
import type { Studio } from '~/types/studio';

const props = defineProps<{
    visible: boolean;
    studio?: Studio | null;
    initialName?: string;
}>();

const emit = defineEmits<{
    close: [];
    updated: [];
    created: [studio: Studio];
}>();

const api = useApi();

const isEditMode = computed(() => !!props.studio);

const form = ref({
    name: '',
    short_name: '',
    url: '',
    description: '',
});

const loading = ref(false);
const error = ref('');
const logoFile = ref<File | null>(null);
const logoPreview = ref<string | null>(null);

const revokeBlobPreview = () => {
    if (logoPreview.value && logoPreview.value.startsWith('blob:')) {
        URL.revokeObjectURL(logoPreview.value);
    }
};

const resetForm = () => {
    form.value = {
        name: props.initialName || '',
        short_name: '',
        url: '',
        description: '',
    };
    revokeBlobPreview();
    logoPreview.value = null;
};

const syncForm = () => {
    if (!props.studio) {
        resetForm();
        return;
    }
    form.value = {
        name: props.studio.name,
        short_name: props.studio.short_name || '',
        url: props.studio.url || '',
        description: props.studio.description || '',
    };
    logoPreview.value = props.studio.logo || null;
};

watch(
    () => props.studio,
    () => {
        syncForm();
    },
    { immediate: true },
);

watch(
    () => props.visible,
    (visible) => {
        if (visible && !props.studio) {
            resetForm();
        }
    },
);

const handleLogoChange = (event: Event) => {
    const input = event.target as HTMLInputElement;
    if (input.files && input.files[0]) {
        revokeBlobPreview();
        logoFile.value = input.files[0];
        logoPreview.value = URL.createObjectURL(input.files[0]);
    }
};

const handleSubmit = async () => {
    error.value = '';
    loading.value = true;

    try {
        if (isEditMode.value && props.studio) {
            // Upload logo first if provided
            if (logoFile.value) {
                await api.uploadStudioLogo(props.studio.id, logoFile.value);
            }
            await api.updateStudio(props.studio.id, form.value);
            emit('updated');
        } else {
            // Create new studio
            let newStudio = await api.createStudio(form.value);
            // Upload logo if provided — use the returned studio which has logo set
            if (logoFile.value && newStudio.id) {
                newStudio = await api.uploadStudioLogo(newStudio.id, logoFile.value);
            }
            emit('created', newStudio);
        }
    } catch (e: unknown) {
        error.value =
            e instanceof Error
                ? e.message
                : `Failed to ${isEditMode.value ? 'update' : 'create'} studio`;
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    error.value = '';
    revokeBlobPreview();
    logoFile.value = null;
    emit('close');
};
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/70
                p-4 backdrop-blur-sm"
            @click.self="handleClose"
        >
            <div class="glass-panel border-border my-8 w-full max-w-lg border p-6">
                <h3 class="mb-4 text-sm font-semibold text-white">
                    {{ isEditMode ? 'Edit Studio' : 'Create Studio' }}
                </h3>

                <div
                    v-if="error"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ error }}
                </div>

                <form class="space-y-4" @submit.prevent="handleSubmit">
                    <!-- Logo upload -->
                    <div class="flex items-start gap-4">
                        <div
                            class="bg-surface border-border relative h-24 w-24 shrink-0
                                overflow-hidden rounded-lg border"
                        >
                            <img
                                v-if="logoPreview"
                                :src="logoPreview"
                                class="h-full w-full object-contain p-2"
                                alt="Studio preview"
                            />
                            <div
                                v-else
                                class="text-dim flex h-full w-full items-center justify-center"
                            >
                                <Icon name="heroicons:building-office-2" size="32" />
                            </div>
                        </div>
                        <div class="flex-1">
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Logo
                            </label>
                            <input
                                type="file"
                                accept="image/*"
                                class="border-border bg-void/80 file:bg-panel file:text-dim w-full
                                    rounded-lg border px-3 py-2 text-sm text-white file:mr-3
                                    file:rounded-lg file:border-0 file:px-3 file:py-1 file:text-xs"
                                @change="handleLogoChange"
                            />
                        </div>
                    </div>

                    <!-- Basic Info -->
                    <div class="grid grid-cols-2 gap-3">
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Name *
                            </label>
                            <input
                                v-model="form.name"
                                type="text"
                                required
                                class="border-border bg-void/80 placeholder-dim/50
                                    focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg border
                                    px-3 py-2 text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                            />
                        </div>
                        <div>
                            <label
                                class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                    uppercase"
                            >
                                Short Name
                            </label>
                            <input
                                v-model="form.short_name"
                                type="text"
                                class="border-border bg-void/80 placeholder-dim/50
                                    focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg border
                                    px-3 py-2 text-sm text-white transition-all focus:ring-1
                                    focus:outline-none"
                            />
                        </div>
                    </div>

                    <!-- URL -->
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Website URL
                        </label>
                        <input
                            v-model="form.url"
                            type="url"
                            placeholder="https://..."
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none"
                        />
                    </div>

                    <!-- Description -->
                    <div>
                        <label
                            class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                                uppercase"
                        >
                            Description
                        </label>
                        <textarea
                            v-model="form.description"
                            rows="3"
                            class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                                focus:ring-lava/20 w-full rounded-lg border px-3 py-2 text-sm
                                text-white transition-all focus:ring-1 focus:outline-none"
                        ></textarea>
                    </div>

                    <!-- Actions -->
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
                            :disabled="loading || !form.name"
                            class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                font-semibold text-white transition-all disabled:cursor-not-allowed
                                disabled:opacity-40"
                        >
                            {{
                                loading
                                    ? 'Saving...'
                                    : isEditMode
                                      ? 'Save Changes'
                                      : 'Create Studio'
                            }}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    </Teleport>
</template>
