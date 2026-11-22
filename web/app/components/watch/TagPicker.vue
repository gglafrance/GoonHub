<script setup lang="ts">
import type { Tag } from '~/types/tag';

const props = defineProps<{
    visible: boolean;
    tags: Tag[];
    anchorEl: HTMLElement | null;
}>();

const emit = defineEmits<{
    select: [id: number];
    close: [];
}>();

const settingsStore = useSettingsStore();

const searchQuery = ref('');
const sortMode = ref<'az' | 'za' | 'most' | 'least'>(settingsStore.defaultTagSort);

const sortLabels: Record<string, string> = {
    az: 'A-Z',
    za: 'Z-A',
    most: 'Most used',
    least: 'Least used',
};

function cycleSortMode() {
    const modes: Array<'az' | 'za' | 'most' | 'least'> = ['az', 'za', 'most', 'least'];
    const idx = modes.indexOf(sortMode.value);
    sortMode.value = modes[(idx + 1) % modes.length] || 'az';
}

const filteredTags = computed(() => {
    let tags = [...props.tags];

    if (searchQuery.value) {
        const q = searchQuery.value.toLowerCase();
        tags = tags.filter((t) => t.name.toLowerCase().includes(q));
    }

    switch (sortMode.value) {
        case 'az':
            tags.sort((a, b) => a.name.localeCompare(b.name));
            break;
        case 'za':
            tags.sort((a, b) => b.name.localeCompare(a.name));
            break;
        case 'most':
            tags.sort((a, b) => b.scene_count - a.scene_count || a.name.localeCompare(b.name));
            break;
        case 'least':
            tags.sort((a, b) => a.scene_count - b.scene_count || a.name.localeCompare(b.name));
            break;
    }

    return tags;
});

const dropdownStyle = ref<{ top: string; left: string }>({ top: '0px', left: '0px' });
const dropdownRef = ref<HTMLElement | null>(null);
const searchInputRef = ref<HTMLInputElement | null>(null);
const listRef = ref<HTMLElement | null>(null);

// The tag Enter adds: the first match, moved with the arrow keys
const highlighted = ref(0);
watch(searchQuery, () => {
    highlighted.value = 0;
});
watch(filteredTags, (tags) => {
    if (highlighted.value > tags.length - 1) highlighted.value = Math.max(0, tags.length - 1);
});

function scrollToHighlighted() {
    nextTick(() => {
        listRef.value
            ?.querySelector(`[data-index="${highlighted.value}"]`)
            ?.scrollIntoView({ block: 'nearest' });
    });
}

// A tag picked (click or Enter): the search is cleared and keeps the focus,
// the picker stays open for the next tag
function selectTag(id: number) {
    emit('select', id);
    searchQuery.value = '';
    highlighted.value = 0;
    nextTick(() => searchInputRef.value?.focus());
}

// Keyboard: arrows move through the matches, Enter adds the highlighted one,
// Escape closes
function onSearchKeydown(event: KeyboardEvent) {
    const count = filteredTags.value.length;
    if (event.key === 'ArrowDown') {
        event.preventDefault();
        if (count) highlighted.value = Math.min(highlighted.value + 1, count - 1);
        scrollToHighlighted();
    } else if (event.key === 'ArrowUp') {
        event.preventDefault();
        highlighted.value = Math.max(highlighted.value - 1, 0);
        scrollToHighlighted();
    } else if (event.key === 'Enter') {
        event.preventDefault();
        const tag = filteredTags.value[highlighted.value];
        if (tag) selectTag(tag.id);
    } else if (event.key === 'Escape') {
        event.preventDefault();
        emit('close');
    }
}
let clickTimeout: ReturnType<typeof setTimeout> | null = null;

function updatePosition() {
    if (!props.anchorEl) return;
    const rect = props.anchorEl.getBoundingClientRect();
    dropdownStyle.value = {
        top: `${rect.bottom + 6}px`,
        left: `${rect.left}px`,
    };
}

function onClickOutside(event: MouseEvent) {
    const target = event.target as Node;
    if (
        dropdownRef.value &&
        !dropdownRef.value.contains(target) &&
        props.anchorEl &&
        !props.anchorEl.contains(target)
    ) {
        emit('close');
    }
}

watch(
    () => props.visible,
    (open) => {
        if (open) {
            updatePosition();
            // ready to type: the search field gets the focus as the picker opens
            nextTick(() => searchInputRef.value?.focus());
            clickTimeout = setTimeout(() => document.addEventListener('click', onClickOutside), 0);
            window.addEventListener('scroll', updatePosition, true);
        } else {
            if (clickTimeout) {
                clearTimeout(clickTimeout);
                clickTimeout = null;
            }
            document.removeEventListener('click', onClickOutside);
            window.removeEventListener('scroll', updatePosition, true);
            searchQuery.value = '';
            highlighted.value = 0;
        }
    },
);

onBeforeUnmount(() => {
    if (clickTimeout) {
        clearTimeout(clickTimeout);
        clickTimeout = null;
    }
    document.removeEventListener('click', onClickOutside);
    window.removeEventListener('scroll', updatePosition, true);
});
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            ref="dropdownRef"
            class="border-border bg-panel fixed z-[9999] min-w-48 rounded-lg border shadow-xl"
            :style="dropdownStyle"
        >
            <!-- Search + Sort header -->
            <div class="border-border/50 flex items-center gap-1 border-b px-2 py-1.5">
                <input
                    ref="searchInputRef"
                    v-model="searchQuery"
                    type="text"
                    placeholder="Search tags..."
                    class="min-w-0 flex-1 bg-transparent text-[11px] text-white/80
                        placeholder-white/30 outline-none"
                    @click.stop
                    @keydown="onSearchKeydown"
                />
                <button
                    class="text-dim shrink-0 rounded px-1.5 py-0.5 text-[9px] transition-colors
                        hover:bg-white/5 hover:text-white/80"
                    :title="`Sort: ${sortLabels[sortMode]}`"
                    @click.stop="cycleSortMode()"
                >
                    {{ sortLabels[sortMode] }}
                </button>
            </div>

            <div v-if="filteredTags.length === 0" class="px-3 py-2">
                <p class="text-dim text-[11px]">
                    {{ searchQuery ? 'No matching tags' : 'No more tags available' }}
                </p>
            </div>
            <div v-else ref="listRef" class="max-h-48 overflow-y-auto py-1">
                <button
                    v-for="(tag, index) in filteredTags"
                    :key="tag.id"
                    :data-index="index"
                    class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-[11px]
                        transition-colors"
                    :class="
                        index === highlighted
                            ? 'bg-white/10 text-white'
                            : 'text-white/70 hover:bg-white/5 hover:text-white'
                    "
                    @mouseenter="highlighted = index"
                    @click="selectTag(tag.id)"
                >
                    <span
                        class="inline-block h-2 w-2 shrink-0 rounded-full"
                        :style="{ backgroundColor: tag.color }"
                    />
                    <span class="flex-1 truncate">{{ tag.name }}</span>
                    <span class="text-[10px] text-white/30">({{ tag.scene_count }})</span>
                </button>
            </div>
        </div>
    </Teleport>
</template>
