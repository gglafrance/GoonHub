<script setup lang="ts">
import type { Tag } from '~/types/tag';

// Tag selector bound to the tags config key (tag names; a scene matches any of them).
const config = defineModel<Record<string, unknown>>({ required: true });

const { fetchTags } = useApiTags();
const allTags = ref<Tag[]>([]);
const query = ref('');

onMounted(async () => {
    try {
        const res = await fetchTags();
        allTags.value = res.data || [];
    } catch {
        allTags.value = [];
    }
});

const selected = computed<string[]>(() =>
    Array.isArray(config.value.tags) ? (config.value.tags as string[]) : [],
);

const suggestions = computed(() => {
    const unselected = allTags.value.filter((t) => !selected.value.includes(t.name));
    // the tags starting with the typed text first
    return rankByMatch(unselected, query.value, (t) => t.name).slice(0, 12);
});

const setTags = (tags: string[]) => {
    const next = { ...config.value };
    if (tags.length) next.tags = tags;
    else delete next.tags;
    config.value = next;
};

const add = (name: string) => {
    setTags([...selected.value, name]);
    query.value = '';
};

const remove = (name: string) => setTags(selected.value.filter((t) => t !== name));
</script>

<template>
    <SettingsHomepageWidgetConfigScenesField
        label="Tags"
        hint="Scenes having at least one of these tags (none: any scene)."
    >
        <div v-if="selected.length" class="mb-1.5 flex flex-wrap gap-1">
            <button
                v-for="tag in selected"
                :key="tag"
                class="border-lava/30 bg-lava/10 text-lava flex items-center gap-1 rounded-md border
                    px-1.5 py-0.5 text-[11px]"
                @click="remove(tag)"
            >
                {{ tag }}
                <Icon name="heroicons:x-mark" size="10" />
            </button>
        </div>
        <input
            v-model="query"
            type="text"
            placeholder="Search tags..."
            class="border-border bg-void/80 focus:border-lava/40 w-full rounded-lg border px-3 py-2
                text-xs text-white focus:outline-none"
        />
        <div v-if="query || !selected.length" class="mt-1.5 flex flex-wrap gap-1">
            <button
                v-for="tag in suggestions"
                :key="tag.id"
                class="border-border text-dim hover:border-lava/30 rounded-md border px-1.5 py-0.5
                    text-[11px] transition-colors hover:text-white"
                @click="add(tag.name)"
            >
                {{ tag.name }}
            </button>
        </div>
    </SettingsHomepageWidgetConfigScenesField>
</template>
