<script setup lang="ts">
import type { EntityConflict } from '~/types/conflict';

// Aliases of an actor or a studio on its page: chips, editable in place by admins
// (each add/remove is saved at once, in strict mode).
const props = defineProps<{
    kind: 'actor' | 'studio';
    entityId: number;
    aliases: string[];
    editable: boolean;
}>();

const emit = defineEmits<{
    updated: [aliases: string[]];
}>();

const { updateActor } = useApiActors();
const { updateStudio } = useApiStudios();

const adding = ref(false);
const newAlias = ref('');
const saving = ref(false);
const error = ref('');
const conflict = ref<EntityConflict<{ uuid: string; name: string }> | null>(null);
const inputRef = ref<HTMLInputElement | null>(null);

const save = async (aliases: string[]) => {
    saving.value = true;
    error.value = '';
    conflict.value = null;
    try {
        const update = props.kind === 'actor' ? updateActor : updateStudio;
        const saved = await update(props.entityId, { aliases }, { strict: true });
        emit('updated', saved.aliases || []);
        return true;
    } catch (e: unknown) {
        conflict.value = getEntityConflict<{ uuid: string; name: string }>(e);
        if (!conflict.value) {
            error.value = e instanceof Error ? e.message : 'Failed to save aliases';
        }
        return false;
    } finally {
        saving.value = false;
    }
};

const startAdding = () => {
    adding.value = true;
    nextTick(() => inputRef.value?.focus());
};

const cancelAdding = () => {
    adding.value = false;
    newAlias.value = '';
};

const addAlias = async () => {
    const alias = newAlias.value.trim();
    if (!alias) return;
    if (props.aliases.some((a) => a.toLowerCase() === alias.toLowerCase())) {
        cancelAdding();
        return;
    }
    if (await save([...props.aliases, alias])) {
        newAlias.value = '';
        nextTick(() => inputRef.value?.focus());
    }
};

const removeAlias = (alias: string) => {
    save(props.aliases.filter((a) => a !== alias));
};
</script>

<template>
    <div v-if="aliases.length > 0 || editable" class="mt-1.5">
        <div class="flex flex-wrap items-center gap-1.5">
            <span class="text-dim text-[11px] tracking-wider uppercase">Also known as</span>
            <span
                v-for="alias in aliases"
                :key="alias"
                class="border-border bg-surface inline-flex items-center gap-1 rounded-full border
                    px-2 py-0.5 text-xs text-white/80"
            >
                {{ alias }}
                <button
                    v-if="editable"
                    type="button"
                    :disabled="saving"
                    class="text-dim hover:text-lava transition-colors disabled:opacity-40"
                    :title="`Remove ${alias}`"
                    @click="removeAlias(alias)"
                >
                    <Icon name="heroicons:x-mark" size="11" />
                </button>
            </span>
            <span v-if="aliases.length === 0" class="text-dim text-xs">No aliases</span>

            <template v-if="editable">
                <form
                    v-if="adding"
                    class="inline-flex items-center gap-1"
                    @submit.prevent="addAlias"
                >
                    <input
                        ref="inputRef"
                        v-model="newAlias"
                        type="text"
                        placeholder="New alias"
                        :disabled="saving"
                        class="border-border bg-void/80 placeholder-dim/50 focus:border-lava/40 h-6
                            w-36 rounded-full border px-2.5 text-xs text-white focus:outline-none"
                        @keydown.esc="cancelAdding"
                    />
                    <button
                        type="submit"
                        :disabled="saving || !newAlias.trim()"
                        class="text-dim hover:text-lava transition-colors disabled:opacity-40"
                        title="Add alias"
                    >
                        <Icon name="heroicons:check" size="14" />
                    </button>
                    <button
                        type="button"
                        class="text-dim transition-colors hover:text-white"
                        title="Cancel"
                        @click="cancelAdding"
                    >
                        <Icon name="heroicons:x-mark" size="14" />
                    </button>
                </form>
                <button
                    v-else
                    type="button"
                    class="border-border text-dim hover:border-lava/50 hover:text-lava inline-flex
                        items-center gap-0.5 rounded-full border border-dashed px-2 py-0.5 text-xs
                        transition-colors"
                    @click="startAdding"
                >
                    <Icon name="heroicons:plus" size="11" />
                    Alias
                </button>
            </template>
        </div>
        <AliasesConflictNotice v-if="conflict" :conflict="conflict" :kind="kind" class="mt-2" />
        <p v-else-if="error" class="text-lava mt-1 text-xs">{{ error }}</p>
    </div>
</template>
