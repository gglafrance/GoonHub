<script setup lang="ts">
// Start of an infinite scroll list opened on a later page: loads the pages before it
defineProps<{
    /** First page in the list */
    firstPage: number;
    loading: boolean;
}>();

const emit = defineEmits<{
    loadPrevious: [];
}>();
</script>

<template>
    <div class="mb-3 flex justify-center">
        <button
            type="button"
            :disabled="loading"
            class="border-border bg-surface text-dim hover:border-border-hover flex items-center
                gap-1.5 rounded-lg border px-4 py-2 text-xs font-medium transition-all
                hover:text-white disabled:opacity-50"
            @click="emit('loadPrevious')"
        >
            <Icon name="heroicons:arrow-up" size="13" />
            Load previous scenes (page {{ firstPage - 1 }})
        </button>
    </div>
</template>
