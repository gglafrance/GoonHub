<script setup lang="ts">
// Multi-select of marker labels (or scene tags) for the marker widget config forms.
const props = defineProps<{
    source?: 'labels' | 'tags';
    placeholder?: string;
}>();

const selected = defineModel<string[]>({ default: () => [] });

const { fetchLabelSuggestions } = useApiMarkers();
const { fetchTags } = useApiTags();

const options = ref<{ value: string; count?: number }[]>([]);
const query = ref('');
const loading = ref(false);

const load = async () => {
    loading.value = true;
    try {
        if (props.source === 'tags') {
            const res = await fetchTags();
            options.value = ((res?.data ?? []) as { name: string }[]).map((t) => ({
                value: t.name,
            }));
        } else {
            const res = await fetchLabelSuggestions();
            options.value = (res?.labels ?? []).map((l) => ({ value: l.label, count: l.count }));
        }
    } catch {
        options.value = [];
    } finally {
        loading.value = false;
    }
};

watch(() => props.source, load);
onMounted(load);

const filtered = computed(() => {
    const q = query.value.trim().toLowerCase();
    return options.value.filter(
        (o) => !selected.value.includes(o.value) && (!q || o.value.toLowerCase().includes(q)),
    );
});

const add = (value: string) => {
    selected.value = [...selected.value, value];
    query.value = '';
};

const remove = (value: string) => {
    selected.value = selected.value.filter((v) => v !== value);
};
</script>

<template>
    <div>
        <div v-if="selected.length" class="mb-1.5 flex flex-wrap gap-1">
            <span
                v-for="value in selected"
                :key="value"
                class="bg-lava/10 text-lava flex items-center gap-1 rounded-full px-2 py-0.5
                    text-[11px]"
            >
                {{ value }}
                <button
                    class="hover:text-white"
                    :aria-label="`Remove ${value}`"
                    @click="remove(value)"
                >
                    <Icon name="heroicons:x-mark" size="11" />
                </button>
            </span>
        </div>
        <input
            v-model="query"
            type="text"
            :placeholder="
                placeholder ?? (source === 'tags' ? 'Search tags...' : 'Search labels...')
            "
            class="border-border bg-void/80 focus:border-lava/40 w-full rounded-lg border px-3 py-2
                text-xs text-white focus:outline-none"
        />
        <div
            v-if="query || filtered.length"
            class="border-border mt-1 max-h-36 overflow-y-auto rounded-lg border p-1"
        >
            <div v-if="loading" class="text-dim px-2 py-1 text-[11px]">Loading...</div>
            <div v-else-if="filtered.length === 0" class="text-dim px-2 py-1 text-[11px]">
                No matches
            </div>
            <button
                v-for="option in filtered.slice(0, 50)"
                :key="option.value"
                class="text-dim flex w-full items-center justify-between rounded px-2 py-1 text-left
                    text-[11px] transition-colors hover:bg-white/5 hover:text-white"
                @click="add(option.value)"
            >
                <span class="truncate">{{ option.value }}</span>
                <span v-if="option.count !== undefined" class="font-mono text-[10px] opacity-50">
                    {{ option.count }}
                </span>
            </button>
        </div>
        <p class="text-dim mt-1 text-[10px]">Leave empty to include all.</p>
    </div>
</template>
