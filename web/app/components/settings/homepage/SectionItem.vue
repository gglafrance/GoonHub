<script setup lang="ts">
import type { HomepageSection, SectionType } from '~/types/homepage';
import { SECTION_TYPE_LABELS, SECTION_ICONS, SECTION_COLORS } from '~/types/homepage';

const props = defineProps<{
    section: HomepageSection;
    isFirst: boolean;
    isLast: boolean;
}>();

const emit = defineEmits<{
    edit: [section: HomepageSection];
    delete: [sectionId: string];
    toggle: [sectionId: string];
    move: [sectionId: string, direction: 'up' | 'down'];
}>();

const typeLabel = computed(
    () => SECTION_TYPE_LABELS[props.section.type as SectionType] || props.section.type,
);

const typeIcon = computed(
    () => SECTION_ICONS[props.section.type as SectionType] || 'heroicons:squares-2x2',
);

const typeColor = computed(
    () => SECTION_COLORS[props.section.type as SectionType] || 'text-dim bg-white/5',
);

const showDeleteConfirm = ref(false);

function handleDelete() {
    showDeleteConfirm.value = true;
}

function confirmDelete() {
    emit('delete', props.section.id);
    showDeleteConfirm.value = false;
}
</script>

<template>
    <div
        class="group flex items-center gap-4 px-5 py-4 transition-colors"
        :class="section.enabled ? 'hover:bg-white/2' : 'opacity-50'"
    >
        <!-- Reorder Controls -->
        <div class="flex flex-col gap-0.5">
            <button
                @click="emit('move', section.id, 'up')"
                :disabled="isFirst"
                class="text-dim -m-1 rounded p-1 transition-all hover:bg-white/10 hover:text-white
                    disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:bg-transparent"
                title="Move up"
            >
                <Icon name="heroicons:chevron-up" size="14" />
            </button>
            <button
                @click="emit('move', section.id, 'down')"
                :disabled="isLast"
                class="text-dim -m-1 rounded p-1 transition-all hover:bg-white/10 hover:text-white
                    disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:bg-transparent"
                title="Move down"
            >
                <Icon name="heroicons:chevron-down" size="14" />
            </button>
        </div>

        <!-- Type Icon -->
        <div
            class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg transition-opacity"
            :class="[typeColor, { 'opacity-50': !section.enabled }]"
        >
            <Icon :name="typeIcon" size="18" />
        </div>

        <!-- Content -->
        <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
                <span class="truncate text-sm font-medium text-white">
                    {{ section.title }}
                </span>
                <span
                    v-if="!section.enabled"
                    class="text-dim shrink-0 rounded bg-white/10 px-1.5 py-0.5 text-[10px]
                        font-medium"
                >
                    Hidden
                </span>
            </div>
            <div class="text-dim mt-0.5 flex items-center gap-2 text-[11px]">
                <span>{{ typeLabel }}</span>
                <span class="text-border">&bull;</span>
                <span>{{ section.limit }} items</span>
                <template v-if="section.sort">
                    <span class="text-border">&bull;</span>
                    <span class="truncate">{{
                        section.sort
                            .replace('_', ' ')
                            .replace('desc', 'descending')
                            .replace('asc', 'ascending')
                    }}</span>
                </template>
            </div>
        </div>

        <!-- Actions -->
        <div class="flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
            <!-- Toggle Visibility -->
            <button
                @click="emit('toggle', section.id)"
                class="text-dim flex items-center justify-center rounded-lg p-2 transition-all
                    hover:bg-white/10 hover:text-white"
                :title="section.enabled ? 'Hide section' : 'Show section'"
            >
                <Icon :name="section.enabled ? 'heroicons:eye' : 'heroicons:eye-slash'" size="16" />
            </button>

            <!-- Edit -->
            <button
                @click="emit('edit', section)"
                class="text-dim flex items-center justify-center rounded-lg p-2 transition-all
                    hover:bg-white/10 hover:text-white"
                title="Edit section"
            >
                <Icon name="heroicons:pencil-square" size="16" />
            </button>

            <!-- Delete -->
            <button
                @click="handleDelete"
                class="hover:text-lava hover:bg-lava/10 text-dim flex items-center justify-center
                    rounded-lg p-2 transition-all"
                title="Delete section"
            >
                <Icon name="heroicons:trash" size="16" />
            </button>
        </div>

        <!-- Delete Confirmation -->
        <Teleport to="body">
            <Transition
                enter-active-class="transition-all duration-200"
                enter-from-class="opacity-0"
                leave-active-class="transition-all duration-150"
                leave-to-class="opacity-0"
            >
                <div
                    v-if="showDeleteConfirm"
                    class="fixed inset-0 z-50 flex items-center justify-center p-4"
                >
                    <div
                        class="bg-void/80 absolute inset-0 backdrop-blur-sm"
                        @click="showDeleteConfirm = false"
                    ></div>
                    <div
                        class="bg-surface border-border relative z-10 w-full max-w-sm rounded-xl
                            border shadow-2xl"
                    >
                        <div class="p-5 text-center">
                            <div
                                class="bg-lava/10 mx-auto mb-4 flex h-12 w-12 items-center
                                    justify-center rounded-full"
                            >
                                <Icon name="heroicons:trash" size="24" class="text-lava" />
                            </div>
                            <h3 class="mb-2 text-sm font-semibold text-white">Delete Section?</h3>
                            <p class="text-dim text-xs">
                                Are you sure you want to delete "{{ section.title }}"? This action
                                cannot be undone.
                            </p>
                        </div>
                        <div class="border-border flex gap-3 border-t p-4">
                            <button
                                @click="showDeleteConfirm = false"
                                class="border-border flex-1 rounded-lg border py-2.5 text-xs
                                    font-medium text-white transition-colors hover:bg-white/5"
                            >
                                Cancel
                            </button>
                            <button
                                @click="confirmDelete"
                                class="bg-lava hover:bg-lava-glow flex-1 rounded-lg py-2.5 text-xs
                                    font-semibold text-white transition-all"
                            >
                                Delete
                            </button>
                        </div>
                    </div>
                </div>
            </Transition>
        </Teleport>
    </div>
</template>
