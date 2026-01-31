<script setup lang="ts">
import type { Actor } from '~/types/actor';

const props = defineProps<{
    visible: boolean;
    actors: Actor[];
    anchorEl: HTMLElement | null;
}>();

const emit = defineEmits<{
    select: [id: number];
    close: [];
    create: [name: string];
}>();

const searchQuery = ref('');
const sortMode = ref<'az' | 'za' | 'most' | 'least'>('az');

const sortLabels: Record<string, string> = {
    az: 'A-Z',
    za: 'Z-A',
    most: 'Most videos',
    least: 'Least videos',
};

function cycleSortMode() {
    const modes: Array<'az' | 'za' | 'most' | 'least'> = ['az', 'za', 'most', 'least'];
    const idx = modes.indexOf(sortMode.value);
    sortMode.value = modes[(idx + 1) % modes.length] || 'az';
}

const filteredActors = computed(() => {
    let actors = [...props.actors];

    if (searchQuery.value) {
        const q = searchQuery.value.toLowerCase();
        actors = actors.filter((a) => a.name.toLowerCase().includes(q));
    }

    switch (sortMode.value) {
        case 'az':
            actors.sort((a, b) => a.name.localeCompare(b.name));
            break;
        case 'za':
            actors.sort((a, b) => b.name.localeCompare(a.name));
            break;
        case 'most':
            actors.sort(
                (a, b) =>
                    (b.video_count || 0) - (a.video_count || 0) || a.name.localeCompare(b.name),
            );
            break;
        case 'least':
            actors.sort(
                (a, b) =>
                    (a.video_count || 0) - (b.video_count || 0) || a.name.localeCompare(b.name),
            );
            break;
    }

    return actors;
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
            class="border-border bg-panel fixed z-[9999] min-w-56 rounded-lg border shadow-xl"
            :style="dropdownStyle"
        >
            <!-- Search + Sort header -->
            <div class="border-border/50 flex items-center gap-1 border-b px-2 py-1.5">
                <input
                    v-model="searchQuery"
                    type="text"
                    placeholder="Search actors..."
                    class="min-w-0 flex-1 bg-transparent text-[11px] text-white/80
                        placeholder-white/30 outline-none"
                    @click.stop
                />
                <button
                    @click.stop="cycleSortMode()"
                    class="text-dim shrink-0 rounded px-1.5 py-0.5 text-[9px] transition-colors
                        hover:bg-white/5 hover:text-white/80"
                    :title="`Sort: ${sortLabels[sortMode]}`"
                >
                    {{ sortLabels[sortMode] }}
                </button>
            </div>

            <div v-if="filteredActors.length === 0" class="px-3 py-2">
                <p class="text-dim text-[11px]">
                    {{ searchQuery ? 'No matching actors' : 'No more actors available' }}
                </p>
                <button
                    v-if="searchQuery"
                    @click="emit('create', searchQuery)"
                    class="text-lava hover:bg-lava/10 mt-2 flex w-full items-center gap-1.5
                        rounded-md px-2 py-1.5 text-left text-[11px] transition-colors"
                >
                    <Icon name="heroicons:plus" size="14" />
                    Create "{{ searchQuery }}"
                </button>
            </div>
            <div v-else class="max-h-64 overflow-y-auto py-1">
                <button
                    v-for="actor in filteredActors"
                    :key="actor.id"
                    @click="emit('select', actor.id)"
                    class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-[11px]
                        text-white/70 transition-colors hover:bg-white/5 hover:text-white"
                >
                    <div class="bg-surface h-5 w-5 shrink-0 overflow-hidden rounded-full">
                        <img
                            v-if="actor.image_url"
                            :src="actor.image_url"
                            :alt="actor.name"
                            class="h-full w-full object-cover"
                        />
                        <Icon
                            v-else
                            name="heroicons:user"
                            size="12"
                            class="text-dim m-auto mt-0.5"
                        />
                    </div>
                    <span class="flex-1 truncate">{{ actor.name }}</span>
                    <span v-if="actor.video_count" class="text-[10px] text-white/30">
                        ({{ actor.video_count }})
                    </span>
                </button>
            </div>
        </div>
    </Teleport>
</template>
