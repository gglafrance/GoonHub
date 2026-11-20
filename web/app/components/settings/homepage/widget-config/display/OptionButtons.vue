<script setup lang="ts">
// Row of option buttons used by the display widget config forms.
defineProps<{
    label: string;
    hint?: string;
    options: { value: string | number; label: string }[];
}>();

const model = defineModel<string | number>({ required: true });
</script>

<template>
    <div>
        <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
            {{ label }}
        </label>
        <div class="flex flex-wrap gap-1.5">
            <button
                v-for="option in options"
                :key="option.value"
                type="button"
                class="rounded-md border px-2.5 py-1 text-[11px] font-medium transition-all"
                :class="
                    model === option.value
                        ? 'border-lava/40 bg-lava/10 text-lava'
                        : 'border-border text-dim hover:text-white'
                "
                @click="model = option.value"
            >
                {{ option.label }}
            </button>
        </div>
        <p v-if="hint" class="text-dim mt-1.5 text-[10px]">{{ hint }}</p>
    </div>
</template>
