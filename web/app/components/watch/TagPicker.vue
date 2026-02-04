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
            setTimeout(() => document.addEventListener('click', onClickOutside), 0);
            window.addEventListener('scroll', updatePosition, true);
        } else {
            document.removeEventListener('click', onClickOutside);
            window.removeEventListener('scroll', updatePosition, true);
            searchQuery.value = '';
        }
    },
);

onBeforeUnmount(() => {
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
                    v-model="searchQuery"
                    type="text"
                    placeholder="Search tags..."
                    class="min-w-0 flex-1 bg-transparent text-[11px] text-white/80
                        placeholder-white/30 outline-none"
                    @click.stop
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
            <div v-else class="max-h-48 overflow-y-auto py-1">
                <button
                    v-for="tag in filteredTags"
                    :key="tag.id"
                    class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-[11px]
                        text-white/70 transition-colors hover:bg-white/5 hover:text-white"
                    @click="emit('select', tag.id)"
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
