<script setup lang="ts">
const props = defineProps<{
    title: string;
    saved?: boolean;
}>();

const emit = defineEmits<{
    save: [value: string];
}>();

const editing = ref(false);
const editValue = ref(props.title);
const inputRef = ref<HTMLInputElement | null>(null);

const startEditing = () => {
    editValue.value = props.title;
    editing.value = true;
    nextTick(() => inputRef.value?.focus());
};

const save = () => {
    editing.value = false;
    if (editValue.value !== props.title) {
        emit('save', editValue.value);
    }
};

watch(
    () => props.title,
    (newTitle) => {
        if (!editing.value) {
            editValue.value = newTitle;
        }
    },
);
</script>

<template>
    <div class="space-y-1">
        <div class="flex items-center gap-2">
            <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">Title</h3>
            <Transition name="fade">
                <span v-if="saved" class="text-[10px] text-emerald-400/80">Saved</span>
            </Transition>
        </div>

        <input
            v-if="editing"
            ref="inputRef"
            v-model="editValue"
            @blur="save"
            @keydown.enter="($event.target as HTMLInputElement).blur()"
            type="text"
            class="border-border focus:border-lava/50 -mx-2 w-[calc(100%+16px)] rounded-md border
                bg-white/3 px-2 py-1 text-sm text-white transition-colors outline-none"
        />
        <p
            v-else
            @click="startEditing"
            class="text-dim -mx-2 cursor-pointer rounded-md px-2 py-1 text-sm transition-colors
                hover:bg-white/3 hover:text-white"
            :class="{ 'text-white': title }"
        >
            {{ title || 'Untitled' }}
        </p>
    </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
    transition: opacity 0.3s ease;
}
.fade-enter-from,
.fade-leave-to {
    opacity: 0;
}
</style>
