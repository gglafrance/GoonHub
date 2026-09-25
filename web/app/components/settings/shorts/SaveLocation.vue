<script setup lang="ts">
import type { ShortsSettings } from '~/types/shorts';

// The shorts folder field, with where it resolves to and whether that is
// inside a storage path.
defineProps<{
    settings: ShortsSettings;
    usesDefault: boolean;
    error: string;
}>();

const model = defineModel<string>({ required: true });

const emit = defineEmits<{
    reset: [];
}>();
</script>

<template>
    <div class="space-y-1.5">
        <div class="flex items-center justify-between">
            <label class="text-xs font-medium text-white">Save location</label>
            <button
                v-if="!usesDefault"
                class="text-lava/80 hover:text-lava text-[11px] transition-colors"
                @click="emit('reset')"
            >
                Reset to default
            </button>
        </div>
        <input
            v-model="model"
            type="text"
            spellcheck="false"
            :placeholder="settings.default_save_dir"
            class="border-border bg-surface w-full rounded-lg border px-3 py-2 font-mono text-xs
                text-white focus:border-white/20 focus:outline-none"
            :class="{ 'border-lava/50': error }"
        />

        <p v-if="error" class="text-lava text-[11px]">{{ error }}</p>

        <div v-if="settings.effective_save_dir" class="space-y-1 pt-1 text-[11px]">
            <div class="text-dim">
                New shorts go to
                <code class="bg-void/50 rounded px-1.5 py-0.5 text-white/80">{{
                    settings.effective_save_dir
                }}</code>
                <span v-if="!settings.save_dir"> (default)</span>
            </div>
            <div v-if="settings.storage_path" class="text-emerald flex items-center gap-1">
                <Icon name="heroicons:check-circle" size="13" />
                Inside storage path {{ settings.storage_path.name }}
            </div>
            <div v-else class="flex items-center gap-1 text-amber-300">
                <Icon name="heroicons:exclamation-triangle" size="13" />
                Outside all storage paths: the Explorer and scans won't include these files
            </div>
            <div v-if="!settings.writable && settings.problem" class="text-lava flex gap-1">
                <Icon name="heroicons:x-circle" size="13" class="mt-px shrink-0" />
                {{ settings.problem }}
            </div>
        </div>
        <div v-else-if="settings.problem" class="text-lava text-[11px]">
            {{ settings.problem }}
        </div>

        <p class="text-dim text-[11px]">
            Path as seen by the server (inside the container). A folder outside the current mounts
            needs a new volume mount first. Changing this only affects new shorts.
        </p>
    </div>
</template>
