<script setup lang="ts">
// Caption line of the scene widgets, with an optional action button on the right.
defineProps<{
    actionLabel?: string;
    actionIcon?: string;
    busy?: boolean;
}>();

const emit = defineEmits<{
    action: [];
}>();
</script>

<template>
    <div class="mb-2.5 flex min-h-7 items-center justify-between gap-3">
        <div class="text-dim min-w-0 truncate text-[11px]">
            <slot />
        </div>
        <button
            v-if="actionLabel"
            class="border-border hover:border-lava/40 hover:text-lava text-dim flex shrink-0
                items-center gap-1.5 rounded-md border px-2.5 py-1 text-[11px] font-medium
                transition-all disabled:opacity-50"
            :disabled="busy"
            @click="emit('action')"
        >
            <Icon
                :name="actionIcon ?? 'heroicons:arrow-path'"
                size="12"
                :class="{ 'animate-spin': busy }"
            />
            {{ actionLabel }}
        </button>
    </div>
</template>
