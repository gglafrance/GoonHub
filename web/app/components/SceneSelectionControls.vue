<script setup lang="ts">
defineProps<{
    selectMode: boolean;
    hasSelection: boolean;
    isSelectingAll: boolean;
    allPageScenesSelected: boolean;
    allScenesSelected: boolean;
    totalScenes: number;
    /** Taller toggle button to match h-10 inputs (actor/studio detail pages) */
    tall?: boolean;
    /** Hide action buttons on mobile (show only on sm+) */
    hideActionsMobile?: boolean;
    /** Show an extra "Select all recursive" button (e.g. "+ subfolders") */
    showRecursive?: boolean;
    isRecursiveDisabled?: boolean;
}>();

defineEmits<{
    'update:selectMode': [value: boolean];
    deselectAll: [];
    selectPage: [];
    selectAll: [];
    selectRecursive: [];
}>();
</script>

<template>
    <div class="flex items-center gap-2">
        <template v-if="selectMode">
            <!-- Deselect all -->
            <button
                v-if="hasSelection"
                class="text-dim hover:text-lava flex items-center gap-1 text-xs transition-colors"
                :class="{ 'hidden sm:flex': hideActionsMobile }"
                @click="$emit('deselectAll')"
            >
                <Icon name="heroicons:x-circle" size="14" />
                Deselect all
            </button>

            <!-- Select page -->
            <button
                v-if="!allPageScenesSelected"
                class="text-dim hover:text-lava flex items-center gap-1 text-xs transition-colors"
                :class="{ 'hidden sm:flex': hideActionsMobile }"
                @click="$emit('selectPage')"
            >
                <Icon name="heroicons:document-check" size="14" />
                Select page
            </button>

            <!-- Select all -->
            <button
                v-if="!allScenesSelected"
                :disabled="isSelectingAll"
                class="text-lava hover:text-lava/80 flex items-center gap-1 text-xs font-medium
                    transition-colors disabled:opacity-50"
                :class="{ 'hidden sm:flex': hideActionsMobile }"
                @click="$emit('selectAll')"
            >
                <Icon name="heroicons:check-badge" size="14" />
                <template v-if="isSelectingAll">Selecting...</template>
                <template v-else>Select all {{ totalScenes }} scenes</template>
            </button>

            <!-- Recursive select (optional) -->
            <button
                v-if="showRecursive"
                :disabled="isRecursiveDisabled"
                class="text-dim hover:text-lava flex items-center gap-1 text-xs transition-colors
                    disabled:opacity-50"
                @click="$emit('selectRecursive')"
            >
                <Icon name="heroicons:folder-arrow-down" size="14" />
                <template v-if="isRecursiveDisabled">Selecting...</template>
                <template v-else>+ subfolders</template>
            </button>
        </template>

        <!-- Select mode toggle -->
        <button
            class="flex items-center gap-1.5 rounded-lg border text-xs font-medium transition-all"
            :class="[
                selectMode
                    ? 'border-lava/40 bg-lava/10 text-lava'
                    : 'border-border text-dim hover:border-border-hover hover:text-white',
                tall ? 'h-10 px-2.5' : 'px-2.5 py-1',
            ]"
            @click="$emit('update:selectMode', !selectMode)"
        >
            <Icon name="heroicons:check-circle" size="14" />
            Select
        </button>
    </div>
</template>
