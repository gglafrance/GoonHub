<script setup lang="ts">
import type { Emote } from '~/types/together';

// Upload of a server emote: name + PNG/GIF/WebP file. The server checks the size and the
// dimensions (goon2gether.emote_max_bytes / emote_max_dimension); the client only hints.
const emit = defineEmits<{
    uploaded: [emote: Emote];
    error: [message: string];
}>();

const { uploadEmote } = useApiTogether();

const name = ref('');
const file = ref<File | null>(null);
const preview = ref('');
const uploading = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);

const nameInvalid = computed(() => name.value !== '' && !isValidEmoteName(name.value));
const fileProblem = computed(() => (file.value ? emoteFileProblem(file.value) : null));
const canSubmit = computed(
    () => !!file.value && isValidEmoteName(name.value) && !fileProblem.value && !uploading.value,
);

const onFile = (e: Event) => {
    const input = e.target as HTMLInputElement;
    const f = input.files?.[0] ?? null;
    if (preview.value) URL.revokeObjectURL(preview.value);
    file.value = f;
    preview.value = f ? URL.createObjectURL(f) : '';
    // The file name gives a default emote name
    if (f && !name.value) name.value = emoteNameFromFile(f.name);
};

const reset = () => {
    name.value = '';
    file.value = null;
    if (preview.value) URL.revokeObjectURL(preview.value);
    preview.value = '';
    if (fileInput.value) fileInput.value.value = '';
};

onBeforeUnmount(() => {
    if (preview.value) URL.revokeObjectURL(preview.value);
});

const submit = async () => {
    if (!canSubmit.value || !file.value) return;
    uploading.value = true;
    try {
        const emote = await uploadEmote(name.value, file.value);
        reset();
        emit('uploaded', emote);
    } catch (e: unknown) {
        emit('error', e instanceof Error ? e.message : 'Failed to upload the emote');
    } finally {
        uploading.value = false;
    }
};
</script>

<template>
    <form class="flex flex-wrap items-end gap-3" @submit.prevent="submit">
        <div
            class="border-border bg-void flex h-14 w-14 shrink-0 items-center justify-center
                rounded-lg border border-dashed"
        >
            <img v-if="preview" :src="preview" alt="" class="max-h-12 max-w-12 object-contain" />
            <Icon v-else name="heroicons:photo" size="18" class="text-dim" />
        </div>
        <div>
            <label class="text-dim mb-1 block text-[11px] font-medium">Image</label>
            <input
                ref="fileInput"
                type="file"
                accept="image/png,image/gif,image/webp"
                class="text-dim file:border-border file:bg-panel file:hover:border-lava/30 text-xs
                    file:mr-2 file:rounded-md file:border file:px-2 file:py-1 file:text-xs
                    file:text-white"
                @change="onFile"
            />
            <p class="mt-1 text-[10px]" :class="fileProblem ? 'text-lava' : 'text-dim'">
                {{ fileProblem ?? 'PNG, GIF or WebP (animated allowed), small and square' }}
            </p>
        </div>
        <div>
            <label class="text-dim mb-1 block text-[11px] font-medium">Name</label>
            <input
                v-model="name"
                type="text"
                maxlength="32"
                placeholder="pog"
                class="border-border bg-surface w-40 rounded-lg border px-3 py-1.5 font-mono text-xs
                    text-white outline-none"
                :class="nameInvalid ? 'border-lava/50' : 'focus:border-lava/40'"
            />
            <p class="mt-1 text-[10px]" :class="nameInvalid ? 'text-lava' : 'text-dim'">
                2 to 32 letters, digits or _
            </p>
        </div>
        <button
            type="submit"
            class="bg-lava hover:bg-lava/90 mb-4 flex items-center gap-1.5 rounded-lg px-3 py-1.5
                text-xs font-medium text-white transition-colors disabled:opacity-50"
            :disabled="!canSubmit"
        >
            <Icon
                :name="uploading ? 'svg-spinners:90-ring-with-bg' : 'heroicons:arrow-up-tray'"
                size="13"
            />
            Upload
        </button>
    </form>
</template>
