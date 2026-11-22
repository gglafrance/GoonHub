<script setup lang="ts">
import type { Tag } from '~/types/tag';

// Create Tag panel of Settings > Tags.
const emit = defineEmits<{
    created: [tag: Tag];
    error: [message: string];
}>();

const { createTag } = useApiTags();

const name = ref('');
const color = ref<string>(TAG_COLOR_PRESETS[0]);
const creating = ref(false);

async function handleCreate() {
    if (!name.value.trim()) return;
    creating.value = true;
    try {
        const tag = await createTag(name.value.trim(), color.value);
        emit('created', { ...tag, scene_count: 0 });
        name.value = '';
    } catch (e: unknown) {
        emit('error', e instanceof Error ? e.message : 'Failed to create tag');
    } finally {
        creating.value = false;
    }
}
</script>

<template>
    <div class="glass-panel p-5">
        <h3 class="mb-5 text-sm font-semibold text-white">Create Tag</h3>
        <div class="space-y-4">
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Name
                </label>
                <input
                    v-model="name"
                    type="text"
                    placeholder="Tag name"
                    maxlength="100"
                    class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20 w-full
                        max-w-64 rounded-lg border px-3.5 py-2.5 text-sm text-white transition-all
                        focus:ring-1 focus:outline-none"
                    @keydown.enter="handleCreate"
                />
            </div>
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Color
                </label>
                <div class="flex items-center gap-2">
                    <button
                        v-for="preset in TAG_COLOR_PRESETS"
                        :key="preset"
                        class="h-6 w-6 rounded-full border-2 transition-all"
                        :class="
                            color === preset
                                ? 'scale-110 border-white'
                                : 'border-transparent hover:scale-110'
                        "
                        :style="{ backgroundColor: preset }"
                        @click="color = preset"
                    />
                </div>
            </div>
            <button
                :disabled="!name.trim() || creating"
                class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                    text-white transition-all disabled:cursor-not-allowed disabled:opacity-40"
                @click="handleCreate"
            >
                Create Tag
            </button>
        </div>
    </div>
</template>
