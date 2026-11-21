<script setup lang="ts">
// Labelled field of the scene widget config forms.
defineProps<{
    label: string;
    hint?: string;
}>();
</script>

<template>
    <div>
        <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
            {{ label }}
        </label>
        <slot />
        <p v-if="hint" class="text-dim/80 mt-1 text-[10px]">{{ hint }}</p>
    </div>
</template>
