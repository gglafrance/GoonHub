<script setup lang="ts">
const props = withDefaults(
    defineProps<{
        modelValue: number;
        total: number;
        limit: number;
        showPageSizeSelector?: boolean;
        pageSizeOptions?: number[];
        maxLimit?: number;
    }>(),
    {
        showPageSizeSelector: false,
        pageSizeOptions: () => [10, 20, 40, 60, 80, 100],
        maxLimit: 100,
    },
);

const emit = defineEmits<{
    (e: 'update:modelValue', page: number): void;
    (e: 'update:limit', limit: number): void;
}>();

const { keys } = useKeyboardLayout();

const totalPages = computed(() => Math.ceil(props.total / props.limit));

const availableSizes = computed(() =>
    props.pageSizeOptions.filter((size) => size <= props.maxLimit),
);

const isCustomLimit = computed(
    () => !availableSizes.value.includes(props.limit) && props.limit >= 1,
);

const editingCustom = ref(false);
const customValue = ref('');
const customInput = ref<HTMLInputElement | null>(null);

const editing = ref(false);
const editValue = ref('');
const pageInput = ref<HTMLInputElement | null>(null);

const setPage = (page: number) => {
    if (page >= 1 && page <= totalPages.value) {
        emit('update:modelValue', page);
    }
};

const handlePageSizeChange = (e: Event) => {
    const val = (e.target as HTMLSelectElement).value;
    if (val === 'custom') {
        customValue.value = String(props.limit);
        editingCustom.value = true;
        nextTick(() => {
            customInput.value?.select();
        });
        return;
    }
    const newLimit = parseInt(val, 10);
    if (!isNaN(newLimit)) {
        emit('update:limit', newLimit);
    }
};

const confirmCustom = () => {
    const parsed = parseInt(customValue.value, 10);
    if (!isNaN(parsed) && parsed >= 1) {
        const clamped = Math.min(parsed, props.maxLimit);
        editingCustom.value = false;
        emit('update:limit', clamped);
    } else {
        editingCustom.value = false;
    }
};

const cancelCustom = () => {
    editingCustom.value = false;
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
    <div
        v-if="totalPages > 1 || (showPageSizeSelector && availableSizes.length > 1)"
        class="flex items-center justify-center gap-2 py-6"
    >
        <template v-if="totalPages > 1">
            <button
                :disabled="modelValue === 1"
                class="border-border bg-surface text-muted hover:border-border-hover rounded-md
                    border px-3 py-1.5 text-[11px] font-medium transition-all hover:text-white
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
                class="border-border bg-surface text-muted hover:border-border-hover rounded-md
                    border px-3 py-1.5 text-[11px] font-medium transition-all hover:text-white
                    disabled:cursor-not-allowed disabled:opacity-30"
                @click="setPage(modelValue + 1)"
            >
                Next
            </button>
        </template>

        <!-- Page size selector -->
        <template v-if="showPageSizeSelector && availableSizes.length > 1">
            <input
                v-if="editingCustom"
                ref="customInput"
                v-model="customValue"
                type="text"
                inputmode="numeric"
                :placeholder="`1-${maxLimit}`"
                class="text-lava bg-surface border-border ml-2 w-16 rounded-md border px-2 py-1.5
                    text-center text-[11px] outline-none focus:border-[#FF4D4D]"
                @keydown.enter="confirmCustom"
                @keydown.escape="cancelCustom"
                @blur="confirmCustom"
            />
            <select
                v-else
                :value="isCustomLimit ? 'current-custom' : limit"
                class="border-border bg-surface text-dim ml-2 rounded-md border px-2 py-1.5
                    text-[11px] transition-all focus:outline-none"
                @change="handlePageSizeChange"
            >
                <option v-if="isCustomLimit" value="current-custom" disabled>
                    {{ limit }} / page
                </option>
                <option v-for="size in availableSizes" :key="size" :value="size">
                    {{ size }} / page
                </option>
                <option value="custom">Custom...</option>
            </select>
        </template>
    </div>
</template>
