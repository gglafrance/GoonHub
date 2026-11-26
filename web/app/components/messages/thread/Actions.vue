<script setup lang="ts">
// Hover actions of a message: add a reaction, and for my messages edit and delete (confirmed).
defineProps<{
    mine: boolean;
    canReact: boolean;
    canEdit: boolean;
}>();
const emit = defineEmits<{
    react: [anchor: HTMLElement];
    edit: [];
    delete: [];
}>();

const confirmDelete = ref(false);

const doDelete = () => {
    confirmDelete.value = false;
    emit('delete');
};
</script>

<template>
    <div
        class="flex shrink-0 items-center opacity-0 transition-opacity group-hover:opacity-100
            focus-within:opacity-100"
    >
        <button
            v-if="canReact"
            type="button"
            class="text-dim hover:text-lava flex h-6 w-6 items-center justify-center rounded"
            title="Add reaction"
            @click="emit('react', $event.currentTarget as HTMLElement)"
        >
            <Icon name="heroicons:face-smile" size="12" />
        </button>
        <template v-if="mine">
            <button
                v-if="canEdit"
                type="button"
                class="text-dim flex h-6 w-6 items-center justify-center rounded hover:text-white"
                title="Edit"
                @click="emit('edit')"
            >
                <Icon name="heroicons:pencil" size="11" />
            </button>
            <button
                v-if="!confirmDelete"
                type="button"
                class="text-dim hover:text-lava flex h-6 w-6 items-center justify-center rounded"
                title="Delete"
                @click="confirmDelete = true"
            >
                <Icon name="heroicons:trash" size="11" />
            </button>
            <template v-else>
                <button
                    type="button"
                    class="bg-lava/15 text-lava rounded px-1.5 py-0.5 text-[10px] font-medium"
                    @click="doDelete"
                >
                    Delete
                </button>
                <button
                    type="button"
                    class="text-dim px-1 text-[10px] hover:text-white"
                    @click="confirmDelete = false"
                >
                    Keep
                </button>
            </template>
        </template>
    </div>
</template>
