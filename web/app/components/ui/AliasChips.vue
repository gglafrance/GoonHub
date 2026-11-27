<script setup lang="ts">
// Editable list of aliases (chips with add/remove), v-model on the list.
const props = withDefaults(
    defineProps<{
        modelValue: string[];
        placeholder?: string;
        disabled?: boolean;
    }>(),
    { placeholder: 'Add alias...', disabled: false },
);

const emit = defineEmits<{
    'update:modelValue': [aliases: string[]];
}>();

const newAlias = ref('');

const addAlias = () => {
    const alias = newAlias.value.trim();
    newAlias.value = '';
    if (!alias) return;
    const lower = alias.toLowerCase();
    if (props.modelValue.some((a) => a.toLowerCase() === lower)) return;
    emit('update:modelValue', [...props.modelValue, alias]);
};

const removeAlias = (index: number) => {
    emit(
        'update:modelValue',
        props.modelValue.filter((_, i) => i !== index),
    );
};
</script>

<template>
    <div>
        <div class="flex gap-2">
            <input
                v-model="newAlias"
                type="text"
                :placeholder="placeholder"
                :disabled="disabled"
                class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40
                    focus:ring-lava/20 flex-1 rounded-lg border px-3 py-2 text-sm text-white
                    transition-all focus:ring-1 focus:outline-none disabled:opacity-40"
                @keydown.enter.prevent="addAlias"
            />
            <button
                type="button"
                :disabled="disabled || !newAlias.trim()"
                class="border-border bg-panel hover:border-lava/50 hover:text-lava text-dim shrink-0
                    rounded-lg border px-3 py-2 text-sm transition-colors
                    disabled:cursor-not-allowed disabled:opacity-40"
                @click="addAlias"
            >
                Add
            </button>
        </div>
        <div v-if="modelValue.length > 0" class="mt-2 flex flex-wrap gap-1.5">
            <span
                v-for="(alias, index) in modelValue"
                :key="alias"
                class="border-border bg-surface inline-flex items-center gap-1 rounded-full border
                    px-2.5 py-1 text-xs text-white"
            >
                {{ alias }}
                <button
                    type="button"
                    :disabled="disabled"
                    class="text-dim hover:text-lava transition-colors"
                    :title="`Remove ${alias}`"
                    @click="removeAlias(index)"
                >
                    <Icon name="heroicons:x-mark" size="12" />
                </button>
            </span>
        </div>
    </div>
</template>
