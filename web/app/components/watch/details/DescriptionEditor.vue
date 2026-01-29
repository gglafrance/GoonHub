<script setup lang="ts">
const props = defineProps<{
    description: string;
}>();

const emit = defineEmits<{
    save: [value: string];
}>();

const editing = ref(false);
const editValue = ref(props.description);
const textareaRef = ref<HTMLTextAreaElement | null>(null);

const autoResize = (event: Event) => {
    const el = event.target as HTMLTextAreaElement;
    el.style.height = 'auto';
    el.style.height = el.scrollHeight + 'px';
};

const startEditing = () => {
    editValue.value = props.description;
    editing.value = true;
    nextTick(() => {
        if (textareaRef.value) {
            textareaRef.value.focus();
            autoResize({ target: textareaRef.value } as unknown as Event);
        }
    });
};

const save = () => {
    editing.value = false;
    if (editValue.value !== props.description) {
        emit('save', editValue.value);
    }
};

watch(
    () => props.description,
    (newDesc) => {
        if (!editing.value) {
            editValue.value = newDesc;
        }
    },
);
</script>

<template>
    <div class="space-y-1">
        <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">Description</h3>

        <textarea
            v-if="editing"
            ref="textareaRef"
            v-model="editValue"
            @blur="save"
            @input="autoResize"
            rows="2"
            class="border-border focus:border-lava/50 w-full resize-none rounded-md border
                bg-white/5 px-2 py-1.5 text-sm text-white/80 transition-colors outline-none"
        />
        <p
            v-else
            @click="startEditing"
            class="text-dim -mx-2 min-h-8 cursor-pointer rounded-md px-2 py-1.5 text-sm
                whitespace-pre-wrap transition-colors hover:bg-white/3 hover:text-white"
            :class="{ 'text-white/70': description }"
        >
            {{ description || 'No description' }}
        </p>
    </div>
</template>
