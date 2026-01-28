<script setup lang="ts">
const props = defineProps<{
    releaseDate: string | null;
}>();

const emit = defineEmits<{
    save: [value: string | null];
}>();

const editing = ref(false);
const editValue = ref('');
const inputRef = ref<HTMLInputElement | null>(null);

const displayDate = computed(() => {
    if (!props.releaseDate) return 'No release date';
    return props.releaseDate.split('T')[0];
});

const startEditing = () => {
    editValue.value = props.releaseDate?.split('T')[0] || '';
    editing.value = true;
    nextTick(() => inputRef.value?.focus());
};

const save = () => {
    editing.value = false;
    const current = props.releaseDate?.split('T')[0] || '';
    if (editValue.value !== current) {
        emit('save', editValue.value || null);
    }
};

watch(
    () => props.releaseDate,
    (newDate) => {
        if (!editing.value) {
            editValue.value = newDate?.split('T')[0] || '';
        }
    },
);
</script>

<template>
    <div class="space-y-1">
        <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">Release Date</h3>

        <input
            v-if="editing"
            ref="inputRef"
            v-model="editValue"
            @blur="save"
            @keydown.enter="($event.target as HTMLInputElement).blur()"
            type="date"
            class="border-border focus:border-lava/50 -mx-2 w-auto rounded-md border bg-white/3 px-2
                py-1 text-sm text-white transition-colors outline-none"
        />
        <p
            v-else
            @click="startEditing"
            class="text-dim -mx-2 cursor-pointer rounded-md px-2 py-1 text-sm transition-colors
                hover:bg-white/3 hover:text-white"
            :class="{ 'text-white': releaseDate }"
        >
            {{ displayDate }}
        </p>
    </div>
</template>
