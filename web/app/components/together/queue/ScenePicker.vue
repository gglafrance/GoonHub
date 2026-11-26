<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

// Library search inside the room: each result can be queued (or suggested).
defineProps<{
    suggest: boolean;
}>();

const emit = defineEmits<{
    add: [sceneId: number];
    close: [];
}>();

const { searchScenes } = useApiScenes();

const query = ref('');
const results = ref<SceneListItem[]>([]);
const loading = ref(false);
const error = ref('');
const added = ref(new Set<number>());
const inputEl = ref<HTMLInputElement | null>(null);

let timer: ReturnType<typeof setTimeout> | null = null;
let token = 0;

const run = async () => {
    const current = ++token;
    loading.value = true;
    error.value = '';
    try {
        const res = (await searchScenes({
            q: query.value.trim() || undefined,
            page: 1,
            limit: 20,
        })) as { data?: SceneListItem[] };
        if (current !== token) return;
        results.value = res.data ?? [];
    } catch (e: unknown) {
        if (current !== token) return;
        error.value = e instanceof Error ? e.message : 'Search failed';
    } finally {
        if (current === token) loading.value = false;
    }
};

watch(query, () => {
    if (timer) clearTimeout(timer);
    timer = setTimeout(run, 300);
});

onMounted(() => {
    run();
    inputEl.value?.focus();
});
onBeforeUnmount(() => {
    if (timer) clearTimeout(timer);
});

const add = (id: number) => {
    emit('add', id);
    added.value = new Set([...added.value, id]);
};
</script>

<template>
    <div class="border-border bg-void/60 rounded-lg border p-2">
        <div class="mb-2 flex items-center gap-2">
            <div
                class="border-border bg-surface focus-within:border-lava/40 flex flex-1 items-center
                    gap-1.5 rounded-md border px-2"
            >
                <Icon name="heroicons:magnifying-glass" size="12" class="text-dim" />
                <input
                    ref="inputEl"
                    v-model="query"
                    type="text"
                    placeholder="Search the library"
                    class="min-w-0 flex-1 bg-transparent py-1.5 text-xs text-white outline-none"
                    @keydown.esc="emit('close')"
                />
                <Icon
                    v-if="loading"
                    name="svg-spinners:90-ring-with-bg"
                    size="12"
                    class="text-dim"
                />
            </div>
            <button
                class="text-dim flex h-7 w-7 items-center justify-center rounded-md
                    hover:text-white"
                title="Close"
                @click="emit('close')"
            >
                <Icon name="heroicons:x-mark" size="14" />
            </button>
        </div>
        <p v-if="error" class="text-lava px-1 text-[11px]">{{ error }}</p>
        <div class="max-h-64 space-y-0.5 overflow-y-auto">
            <div
                v-for="s in results"
                :key="s.id"
                class="flex items-center gap-2 rounded-md px-1 py-1 hover:bg-white/[0.03]"
            >
                <img
                    :src="`/thumbnails/${s.id}?size=sm`"
                    alt=""
                    class="bg-void h-8 w-14 shrink-0 rounded object-cover"
                    loading="lazy"
                />
                <div class="min-w-0 flex-1">
                    <div class="truncate text-xs text-white" :title="s.title">{{ s.title }}</div>
                    <div class="text-dim font-mono text-[10px]">{{ formatClock(s.duration) }}</div>
                </div>
                <button
                    class="flex h-6 shrink-0 items-center gap-1 rounded-md border px-2 text-[10px]
                        font-medium transition-colors"
                    :class="
                        added.has(s.id)
                            ? 'border-emerald/30 text-emerald'
                            : 'border-border hover:border-lava/30 hover:text-lava text-white'
                    "
                    @click="add(s.id)"
                >
                    <Icon
                        :name="added.has(s.id) ? 'heroicons:check' : 'heroicons:plus'"
                        size="11"
                    />
                    {{ added.has(s.id) ? 'Added' : suggest ? 'Suggest' : 'Queue' }}
                </button>
            </div>
            <p
                v-if="!loading && results.length === 0 && !error"
                class="text-dim py-4 text-center text-[11px]"
            >
                No scene found
            </p>
        </div>
    </div>
</template>
