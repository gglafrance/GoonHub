<script setup lang="ts">
import type { ProfileEdit, ProfileLayoutBlock, ProfileSectionConfig } from '~/types/profile';
import type { Audience, PrivacySettings } from '~/types/social';
import type { BlockPosition } from '~/utils/profileLayout';

// One section in the editor: its template, its place, and its columns of blocks. A block is
// dragged onto another block (inserted before it) or onto the end of a column.
const props = defineProps<{
    section: ProfileSectionConfig;
    index: number;
    sections: ProfileSectionConfig[];
    state: ProfileEdit;
    privacy: PrivacySettings;
    templateOptions: { value: string; label: string }[];
    dragFrom: BlockPosition | null;
    canAdd: boolean;
}>();
const emit = defineEmits<{
    template: [template: string];
    moveSection: [delta: number];
    remove: [];
    dragStart: [at: BlockPosition];
    dragEnd: [];
    drop: [at: BlockPosition];
    moveBlock: [from: BlockPosition, to: BlockPosition];
    setBlock: [at: BlockPosition, block: ProfileLayoutBlock | null];
    addBlock: [column: number, block: ProfileLayoutBlock];
    audience: [item: string, audience: Audience | 'default'];
}>();

const columnClasses = computed(() => sectionColumnClasses(props.section.template));
const over = ref<string | null>(null);

const pos = (column: number, index: number): BlockPosition => ({
    section: props.index,
    column,
    index,
});

function onDragStart(e: DragEvent, at: BlockPosition) {
    if (e.dataTransfer) {
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', 'block');
    }
    emit('dragStart', at);
}

function onDragOver(e: DragEvent, key: string) {
    if (!props.dragFrom) return;
    e.preventDefault();
    over.value = key;
}

function onDrop(e: DragEvent, at: BlockPosition) {
    e.preventDefault();
    e.stopPropagation();
    over.value = null;
    emit('drop', at);
}

function moveBy(at: BlockPosition, direction: 'up' | 'down' | 'left' | 'right') {
    const to = neighbourPosition(props.sections, at, direction);
    if (to) emit('moveBlock', at, to);
}

// Block library: every type; a custom list block needs one of the owner's lists
const addOptions = computed(() => [
    { value: '', label: 'Add a block...' },
    ...props.state.block_types
        .filter((t) => t.type !== 'custom_list' || props.state.lists.length > 0)
        .map((t) => ({ value: t.type, label: t.label })),
]);

// Remounts the "Add a block" select after a pick, back to its placeholder
const addKey = ref(0);
function add(column: number, type: string | number) {
    addKey.value++;
    if (!type) return;
    const config: Record<string, number | string> = {};
    if (type === 'custom_list') config.list_id = props.state.lists[0]?.id ?? 0;
    emit('addBlock', column, {
        type: String(type),
        visible: true,
        locked_placeholder: false,
        config,
    });
}
</script>

<template>
    <div class="border-border rounded-lg border bg-white/[0.01] p-3">
        <div class="mb-2 flex flex-wrap items-center gap-2">
            <span class="text-dim font-mono text-[10px]">#{{ props.index + 1 }}</span>
            <div class="w-44">
                <UiSelectMenu
                    :model-value="props.section.template"
                    :options="props.templateOptions"
                    @update:model-value="emit('template', String($event))"
                />
            </div>
            <div class="ml-auto flex items-center gap-1">
                <button
                    class="text-dim hover:text-white disabled:opacity-30"
                    :disabled="props.index === 0"
                    title="Move the section up"
                    @click="emit('moveSection', -1)"
                >
                    <Icon name="heroicons:arrow-up" size="13" />
                </button>
                <button
                    class="text-dim hover:text-white disabled:opacity-30"
                    :disabled="props.index === props.sections.length - 1"
                    title="Move the section down"
                    @click="emit('moveSection', 1)"
                >
                    <Icon name="heroicons:arrow-down" size="13" />
                </button>
                <button
                    class="text-dim hover:text-lava"
                    title="Remove the section"
                    @click="emit('remove')"
                >
                    <Icon name="heroicons:trash" size="13" />
                </button>
            </div>
        </div>
        <div class="grid grid-cols-1 gap-2 md:grid-cols-12">
            <div
                v-for="(column, ci) in props.section.columns"
                :key="ci"
                class="border-border flex min-w-0 flex-col gap-2 rounded-md border border-dashed
                    p-2"
                :class="[columnClasses[ci], over === `end-${ci}` ? 'border-lava/50 bg-lava/5' : '']"
                @dragover="onDragOver($event, `end-${ci}`)"
                @dragleave="over = null"
                @drop="onDrop($event, pos(ci, column.length))"
            >
                <div
                    v-for="(block, bi) in column"
                    :key="block.id ?? `${block.type}-${bi}`"
                    draggable="true"
                    :class="over === `${ci}-${bi}` ? 'border-lava/60 border-t-2' : ''"
                    @dragstart="onDragStart($event, pos(ci, bi))"
                    @dragend="emit('dragEnd')"
                    @dragover.stop="onDragOver($event, `${ci}-${bi}`)"
                    @drop="onDrop($event, pos(ci, bi))"
                >
                    <ProfileEditBlock
                        :block="block"
                        :state="props.state"
                        :privacy="props.privacy"
                        :can="{
                            up: !!neighbourPosition(props.sections, pos(ci, bi), 'up'),
                            down: !!neighbourPosition(props.sections, pos(ci, bi), 'down'),
                            left: ci > 0,
                            right: ci < props.section.columns.length - 1,
                        }"
                        @update="emit('setBlock', pos(ci, bi), $event)"
                        @remove="emit('setBlock', pos(ci, bi), null)"
                        @move="moveBy(pos(ci, bi), $event)"
                        @audience="(item, audience) => emit('audience', item, audience)"
                    />
                </div>
                <div v-if="props.canAdd" class="mt-auto">
                    <UiSelectMenu
                        :key="addKey"
                        model-value=""
                        :options="addOptions"
                        @update:model-value="add(ci, $event)"
                    />
                </div>
            </div>
        </div>
    </div>
</template>
