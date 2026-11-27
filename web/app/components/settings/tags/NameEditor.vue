<script setup lang="ts">
import type { Tag } from '~/types/tag';

// Inline rename of a tag in the Settings > Tags list: click the name (or the pencil) to edit,
// Enter saves, Escape cancels. A name already used by another tag offers to merge into it.
const props = defineProps<{
    tag: Tag;
}>();

const emit = defineEmits<{
    renamed: [tag: Tag];
    mergeInto: [target: Tag];
}>();

const { updateTag } = useApiTags();

const editing = ref(false);
const value = ref('');
const saving = ref(false);
const error = ref('');
const conflict = ref<Tag | null>(null);
const input = ref<HTMLInputElement | null>(null);

async function start() {
    value.value = props.tag.name;
    error.value = '';
    conflict.value = null;
    editing.value = true;
    await nextTick();
    input.value?.focus();
    input.value?.select();
}

function cancel() {
    editing.value = false;
    error.value = '';
    conflict.value = null;
}

async function save() {
    if (saving.value) return;
    const check = checkTagRename(value.value, props.tag.name);
    if (check.kind === 'unchanged') return cancel();
    if (check.kind === 'invalid') {
        error.value = check.error;
        conflict.value = null;
        return;
    }
    saving.value = true;
    error.value = '';
    conflict.value = null;
    try {
        const saved: Tag = await updateTag(props.tag.id, { name: check.name });
        editing.value = false;
        emit('renamed', { ...props.tag, name: saved.name, color: saved.color });
    } catch (e: unknown) {
        const c = getEntityConflict<Tag>(e);
        conflict.value = c ? c.existing : null;
        error.value = c?.error || (e instanceof Error ? e.message : 'Failed to rename the tag');
        await nextTick();
        input.value?.focus();
    } finally {
        saving.value = false;
    }
}

// leaving the field closes it only when there is nothing to keep (a change or a message)
function onBlur() {
    if (!saving.value && !error.value && value.value.trim() === props.tag.name) cancel();
}

function mergeInto() {
    if (!conflict.value) return;
    const target = conflict.value;
    cancel();
    emit('mergeInto', target);
}
</script>

<template>
    <div class="min-w-0 flex-1">
        <div v-if="!editing" class="flex min-w-0 items-center gap-1.5">
            <button
                class="min-w-0 truncate text-left text-xs whitespace-pre text-white
                    hover:text-white/80"
                title="Rename tag"
                @click="start"
            >
                {{ tag.name }}
            </button>
            <button
                class="text-dim flex shrink-0 items-center opacity-0 transition-all
                    group-hover:opacity-100 hover:text-white"
                title="Rename tag"
                :aria-label="`Rename ${tag.name}`"
                @click="start"
            >
                <Icon name="heroicons:pencil" size="12" />
            </button>
        </div>
        <div v-else>
            <div class="flex items-center gap-2">
                <input
                    ref="input"
                    v-model="value"
                    type="text"
                    :disabled="saving"
                    :aria-label="`New name for ${tag.name}`"
                    class="bg-void/80 focus:ring-lava/20 w-full rounded-md border px-2 py-1 text-xs
                        text-white transition-all focus:ring-1 focus:outline-none
                        disabled:opacity-60"
                    :class="error ? 'border-lava/50' : 'border-border focus:border-lava/40'"
                    @keydown.enter.prevent="save"
                    @keydown.esc.prevent="cancel"
                    @blur="onBlur"
                />
                <LoadingSpinner v-if="saving" size="sm" />
            </div>
            <div v-if="error" class="mt-1 flex flex-wrap items-center gap-2">
                <span class="text-lava text-[11px]">{{ error }}</span>
                <button
                    v-if="conflict"
                    class="border-lava/30 text-lava hover:bg-lava/10 flex items-center gap-1
                        rounded-md border px-2 py-0.5 text-[11px] font-medium transition-all"
                    @mousedown.prevent
                    @click="mergeInto"
                >
                    <Icon name="heroicons:arrows-pointing-in" size="12" />
                    Merge into "{{ conflict.name }}"
                </button>
            </div>
        </div>
    </div>
</template>
