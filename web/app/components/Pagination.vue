<script setup lang="ts">
const props = defineProps<{
    modelValue: number;
    total: number;
    limit: number;
}>();

const emit = defineEmits<{
    (e: 'update:modelValue', page: number): void;
}>();

const { keys } = useKeyboardLayout();

const totalPages = computed(() => Math.ceil(props.total / props.limit));

const setPage = (page: number) => {
    if (page >= 1 && page <= totalPages.value) {
        emit('update:modelValue', page);
    }
};

const handleKeydown = (e: KeyboardEvent) => {
    // Skip if typing in input, textarea, or contenteditable
    const target = e.target as HTMLElement;
    if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
        return;
    }

    if (e.key === keys.value.pagePrev) {
        setPage(props.modelValue - 1);
    } else if (e.key === keys.value.pageNext) {
        setPage(props.modelValue + 1);
    }
};

onMounted(() => {
    window.addEventListener('keydown', handleKeydown);
});

onUnmounted(() => {
    window.removeEventListener('keydown', handleKeydown);
});
</script>

<template>
    <div v-if="totalPages > 1" class="flex items-center justify-center gap-2 py-6">
        <button
            @click="setPage(modelValue - 1)"
            :disabled="modelValue === 1"
            class="border-border bg-surface text-muted hover:border-border-hover rounded-md border
                px-3 py-1.5 text-[11px] font-medium transition-all hover:text-white
                disabled:cursor-not-allowed disabled:opacity-30"
        >
            Prev
        </button>

        <span class="text-dim px-3 font-mono text-[11px]">
            <span class="text-lava font-semibold">{{ modelValue }}</span>
            <span class="mx-1">/</span>
            {{ totalPages }}
        </span>

        <button
            @click="setPage(modelValue + 1)"
            :disabled="modelValue === totalPages"
            class="border-border bg-surface text-muted hover:border-border-hover rounded-md border
                px-3 py-1.5 text-[11px] font-medium transition-all hover:text-white
                disabled:cursor-not-allowed disabled:opacity-30"
        >
            Next
        </button>
    </div>
</template>
