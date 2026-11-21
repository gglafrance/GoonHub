<script setup lang="ts">
import type { SceneTableColumn } from '~/types/settings';

// Chooses the visible scene table columns (saved in the user settings, shared by all pages)
const props = defineProps<{
    tall?: boolean;
    /** Columns this page cannot fill (not offered) */
    hiddenColumns?: SceneTableColumn[];
}>();

const settingsStore = useSettingsStore();

const open = ref(false);
const root = ref<HTMLElement | null>(null);

const visible = computed(() => new Set(settingsStore.sceneListView.columns));
const choices = computed(() =>
    SCENE_TABLE_COLUMNS.filter((c) => !props.hiddenColumns?.includes(c.id)),
);

const toggle = (id: SceneTableColumn) => {
    // The title column always shows: the row needs a link and a name
    if (id === 'title') return;
    const next = new Set(visible.value);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    // Display order is the column order of the table
    settingsStore.setSceneTableColumns(
        SCENE_TABLE_COLUMNS.map((c) => c.id).filter((c) => next.has(c)),
    );
};

const reset = () => settingsStore.setSceneTableColumns([...DEFAULT_SCENE_TABLE_COLUMNS]);

const onClickOutside = (event: MouseEvent) => {
    if (open.value && root.value && !root.value.contains(event.target as Node)) {
        open.value = false;
    }
};

onMounted(() => document.addEventListener('mousedown', onClickOutside));
onUnmounted(() => document.removeEventListener('mousedown', onClickOutside));
</script>

<template>
    <div ref="root" class="relative shrink-0" :class="tall ? 'h-10' : ''">
        <button
            type="button"
            class="border-border bg-panel text-dim flex h-full items-center gap-1.5 rounded-lg
                border px-2.5 py-1.5 text-[11px] font-medium transition-all hover:text-white"
            :class="{ 'border-lava/30 text-lava': open }"
            title="Table columns"
            :aria-expanded="open"
            @click="open = !open"
        >
            <Icon name="heroicons:view-columns" size="14" />
            <span class="hidden sm:inline">Columns</span>
        </button>

        <div
            v-if="open"
            class="border-border bg-panel absolute top-full right-0 z-30 mt-1 w-48 rounded-lg border
                p-1.5 shadow-xl"
        >
            <button
                v-for="col in choices"
                :key="col.id"
                type="button"
                class="flex w-full items-center gap-2 rounded-md px-2 py-1 text-left text-[11px]
                    transition-colors"
                :class="
                    col.id === 'title'
                        ? 'text-dim cursor-default'
                        : 'text-white/80 hover:bg-white/5 hover:text-white'
                "
                :aria-pressed="visible.has(col.id)"
                @click="toggle(col.id)"
            >
                <span
                    class="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded border
                        transition-all"
                    :class="visible.has(col.id) ? 'border-lava bg-lava' : 'border-border bg-void'"
                >
                    <Icon
                        v-if="visible.has(col.id)"
                        name="heroicons:check"
                        size="10"
                        class="text-white"
                    />
                </span>
                {{ col.label }}
            </button>
            <div class="border-border mt-1 border-t px-0.5 pt-1">
                <button
                    type="button"
                    class="text-dim w-full rounded-md px-1.5 py-1 text-left text-[10px]
                        transition-colors hover:text-white"
                    @click="reset"
                >
                    Reset to default
                </button>
            </div>
        </div>
    </div>
</template>
