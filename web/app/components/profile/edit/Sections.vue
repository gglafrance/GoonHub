<script setup lang="ts">
import type { ProfileEdit, ProfileLayoutBlock, ProfileSectionConfig } from '~/types/profile';
import type { Audience, PrivacySettings } from '~/types/social';
import type { BlockPosition } from '~/utils/profileLayout';

// The sections of a tab in the editor: add, remove, reorder, change their column template, and
// move blocks between columns and sections (drag and drop, or the arrows of each block).
const props = defineProps<{
    sections: ProfileSectionConfig[];
    state: ProfileEdit;
    privacy: PrivacySettings;
    allIds: string[];
}>();
const emit = defineEmits<{
    update: [sections: ProfileSectionConfig[]];
    audience: [item: string, audience: Audience | 'default'];
}>();

const limits = computed(() => props.state.limits);
const blockCount = computed(() =>
    props.sections.reduce((n, s) => n + s.columns.reduce((m, c) => m + c.length, 0), 0),
);

const dragFrom = ref<BlockPosition | null>(null);

function update(next: ProfileSectionConfig[]) {
    emit('update', next);
}

function addSection(template: string) {
    update([...props.sections, newSection(newLayoutId('s', props.allIds), template)]);
}

function moveSection(from: number, to: number) {
    update(moveItem(props.sections, from, to));
}

const confirmRemove = ref<number | null>(null);
function removeSection(i: number) {
    const s = props.sections[i];
    if (s && s.columns.some((c) => c.length > 0)) {
        confirmRemove.value = i;
        return;
    }
    update(props.sections.filter((_, j) => j !== i));
}
function confirmRemoveSection() {
    const i = confirmRemove.value;
    confirmRemove.value = null;
    if (i !== null) update(props.sections.filter((_, j) => j !== i));
}

function setTemplate(i: number, template: string) {
    update(props.sections.map((s, j) => (j === i ? setSectionTemplate(s, template) : s)));
}

function moveTo(from: BlockPosition, to: BlockPosition) {
    update(moveBlock(props.sections, from, to));
}

function drop(to: BlockPosition) {
    const from = dragFrom.value;
    dragFrom.value = null;
    if (from) moveTo(from, to);
}

function addBlock(section: number, column: number, block: ProfileLayoutBlock) {
    const next = props.sections.map((s) => ({ ...s, columns: s.columns.map((c) => c.slice()) }));
    next[section]?.columns[column]?.push({ ...block, id: newLayoutId('b', props.allIds) });
    update(next);
}

function setBlock(at: BlockPosition, block: ProfileLayoutBlock | null) {
    const next = props.sections.map((s) => ({ ...s, columns: s.columns.map((c) => c.slice()) }));
    const col = next[at.section]?.columns[at.column];
    if (!col) return;
    if (block) col[at.index] = block;
    else col.splice(at.index, 1);
    update(next);
}

const templateOptions = computed(() =>
    props.state.section_templates.map((t) => ({ value: t.id, label: t.label })),
);
const newTemplate = ref('1');
</script>

<template>
    <div class="space-y-3">
        <div class="text-dim flex items-center gap-2 text-[10px] tracking-wider uppercase">
            Sections
            <span class="font-mono normal-case">
                {{ blockCount }}/{{ limits.max_blocks_per_tab }} blocks
            </span>
        </div>
        <ProfileEditSection
            v-for="(section, i) in props.sections"
            :key="section.id"
            :section="section"
            :index="i"
            :sections="props.sections"
            :state="props.state"
            :privacy="props.privacy"
            :template-options="templateOptions"
            :drag-from="dragFrom"
            :can-add="blockCount < limits.max_blocks_per_tab"
            @template="setTemplate(i, $event)"
            @move-section="moveSection(i, i + $event)"
            @remove="removeSection(i)"
            @drag-start="dragFrom = $event"
            @drag-end="dragFrom = null"
            @drop="drop"
            @move-block="moveTo"
            @set-block="setBlock"
            @add-block="(column, block) => addBlock(i, column, block)"
            @audience="(item, audience) => emit('audience', item, audience)"
        />
        <div class="flex flex-wrap items-center gap-2">
            <div class="w-44">
                <UiSelectMenu v-model="newTemplate" :options="templateOptions" />
            </div>
            <button
                class="border-border text-dim hover:border-lava/40 hover:text-lava flex items-center
                    gap-1 rounded-md border border-dashed px-2.5 py-1.5 text-[11px]
                    transition-colors disabled:opacity-40"
                :disabled="props.sections.length >= limits.max_sections"
                @click="addSection(String(newTemplate))"
            >
                <Icon name="heroicons:plus" size="12" />
                Add a section
                <span class="font-mono"
                    >({{ props.sections.length }}/{{ limits.max_sections }})</span
                >
            </button>
        </div>
        <p class="text-dim text-[10px]">
            Columns stack on small screens. Who can see a block is a privacy setting (also in
            Settings > Privacy); blocks of the same kind share it.
        </p>
        <UiConfirmModal
            :visible="confirmRemove !== null"
            title="Remove this section?"
            confirm-label="Remove"
            @close="confirmRemove = null"
            @confirm="confirmRemoveSection"
        >
            <p class="text-dim text-xs">Its blocks leave the tab when you save.</p>
        </UiConfirmModal>
    </div>
</template>
