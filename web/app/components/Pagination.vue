<script setup lang="ts">
const props = defineProps<{
    modelValue: number;
    total: number;
    limit: number;
}>();

const emit = defineEmits<{
    (e: 'update:modelValue', page: number): void;
}>();

const totalPages = computed(() => Math.ceil(props.total / props.limit));

const setPage = (page: number) => {
    if (page >= 1 && page <= totalPages.value) {
        emit('update:modelValue', page);
    }
};
</script>

<template>
    <div v-if="totalPages > 1" class="flex items-center justify-center space-x-2 py-8">
        <button
            @click="setPage(modelValue - 1)"
            :disabled="modelValue === 1"
            class="rounded-lg bg-white/5 px-4 py-2 text-sm font-medium text-white backdrop-blur-md
                transition hover:bg-white/10 disabled:cursor-not-allowed disabled:opacity-50"
        >
            Previous
        </button>

        <span class="px-4 text-sm text-gray-400">
            Page <span class="text-neon-green font-bold">{{ modelValue }}</span> of {{ totalPages }}
        </span>

        <button
            @click="setPage(modelValue + 1)"
            :disabled="modelValue === totalPages"
            class="rounded-lg bg-white/5 px-4 py-2 text-sm font-medium text-white backdrop-blur-md
                transition hover:bg-white/10 disabled:cursor-not-allowed disabled:opacity-50"
        >
            Next
        </button>
    </div>
</template>
