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

const editing = ref(false);
const editValue = ref('');
const pageInput = ref<HTMLInputElement | null>(null);

const setPage = (page: number) => {
    if (page >= 1 && page <= totalPages.value) {
        emit('update:modelValue', page);
    }
};

const startEditing = () => {
    editValue.value = String(props.modelValue);
    editing.value = true;
    nextTick(() => {
        pageInput.value?.select();
    });
};

const confirmEdit = () => {
    const parsed = parseInt(editValue.value, 10);
    if (!isNaN(parsed)) {
        setPage(Math.min(Math.max(1, parsed), totalPages.value));
    }
    editing.value = false;
};

const cancelEdit = () => {
    editing.value = false;
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
            :disabled="modelValue === 1"
            class="border-border bg-surface text-muted hover:border-border-hover rounded-md border
                px-3 py-1.5 text-[11px] font-medium transition-all hover:text-white
                disabled:cursor-not-allowed disabled:opacity-30"
            @click="setPage(modelValue - 1)"
        >
            Prev
        </button>

        <span class="text-dim px-3 font-mono text-[11px]">
            <input
                v-if="editing"
                ref="pageInput"
                v-model="editValue"
                type="text"
                inputmode="numeric"
                class="text-lava bg-surface border-border w-10 rounded border text-center
                    text-[11px] font-semibold outline-none focus:border-[#FF4D4D]"
                @keydown.enter="confirmEdit"
                @keydown.escape="cancelEdit"
                @blur="confirmEdit"
            />
            <span
                v-else
                class="text-lava cursor-pointer font-semibold hover:underline"
                @click="startEditing"
                >{{ modelValue }}</span
            >
            <span class="mx-1">/</span>
            {{ totalPages }}
        </span>

        <button
            :disabled="modelValue === totalPages"
            class="border-border bg-surface text-muted hover:border-border-hover rounded-md border
                px-3 py-1.5 text-[11px] font-medium transition-all hover:text-white
                disabled:cursor-not-allowed disabled:opacity-30"
            @click="setPage(modelValue + 1)"
        >
            Next
        </button>
    </div>
</template>
