<script setup lang="ts">
// The web page a scene comes from: a link that opens in a new tab, editable
// inline. Saving an empty value clears it. The server checks the URL.
const props = defineProps<{
    sourceUrl: string;
}>();

const emit = defineEmits<{
    save: [value: string];
}>();

const editing = ref(false);
const editValue = ref(props.sourceUrl);
const inputRef = ref<HTMLInputElement | null>(null);

// "www.lpsg.com/threads/..." without the scheme, for display
const shortUrl = computed(() => props.sourceUrl.replace(/^https?:\/\//i, ''));

const startEditing = () => {
    editValue.value = props.sourceUrl;
    editing.value = true;
    nextTick(() => inputRef.value?.focus());
};

const save = () => {
    if (!editing.value) return;
    editing.value = false;
    const value = editValue.value.trim();
    if (value !== props.sourceUrl) {
        emit('save', value);
    }
};

const cancel = () => {
    editing.value = false;
    editValue.value = props.sourceUrl;
};

watch(
    () => props.sourceUrl,
    (url) => {
        if (!editing.value) {
            editValue.value = url;
        }
    },
);
</script>

<template>
    <div class="space-y-1">
        <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">Source</h3>

        <input
            v-if="editing"
            ref="inputRef"
            v-model="editValue"
            type="url"
            inputmode="url"
            spellcheck="false"
            placeholder="https://..."
            class="border-border focus:border-lava/50 w-full rounded-md border bg-white/5 px-2
                py-1.5 text-sm text-white/80 transition-colors outline-none"
            @blur="save"
            @keydown.enter.prevent="save"
            @keydown.esc.prevent="cancel"
        />
        <div v-else-if="sourceUrl" class="group -mx-2 flex min-h-8 items-center gap-1.5 px-2">
            <a
                :href="sourceUrl"
                target="_blank"
                rel="noopener noreferrer nofollow"
                :title="sourceUrl"
                class="hover:text-lava flex min-w-0 items-center gap-1.5 text-sm text-white/70
                    transition-colors"
            >
                <Icon name="heroicons:arrow-top-right-on-square" size="14" class="shrink-0" />
                <span class="truncate">{{ shortUrl }}</span>
            </a>
            <button
                type="button"
                title="Edit the source"
                class="text-dim shrink-0 rounded p-1 opacity-0 transition-opacity
                    group-hover:opacity-100 hover:text-white focus:opacity-100"
                @click="startEditing"
            >
                <Icon name="heroicons:pencil-square" size="14" />
            </button>
        </div>
        <p
            v-else
            class="text-dim -mx-2 min-h-8 cursor-pointer rounded-md px-2 py-1.5 text-sm
                transition-colors hover:bg-white/3 hover:text-white"
            @click="startEditing"
        >
            No source
        </p>
    </div>
</template>
