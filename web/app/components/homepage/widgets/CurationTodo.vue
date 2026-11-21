<script setup lang="ts">
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';

// curation_todo widget: what is left to organize in the library.
interface CurationPayload {
    total: number;
    no_tags: number;
    no_actors: number;
    no_studio: number;
    no_thumbnail: number;
    no_sprites: number;
    watched_not_rated: number;
    is_admin: boolean;
    unresolved_duplicates?: number;
}

interface TodoItem {
    key: string;
    label: string;
    count: number;
    icon: string;
    // Share of the library, when the count is about library scenes
    share?: number;
    link?: string;
    hint?: string;
}

const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData<CurationPayload>;
}>();

defineEmits<{ refresh: [] }>();

const number = new Intl.NumberFormat();
const NuxtLinkComponent = resolveComponent('NuxtLink');

const items = computed<TodoItem[]>(() => {
    const p = props.data.widget;
    if (!p) return [];
    const share = (n: number) => (p.total > 0 ? n / p.total : 0);
    const list: TodoItem[] = [
        {
            key: 'tags',
            label: 'Without tags',
            count: p.no_tags,
            icon: 'heroicons:tag',
            share: share(p.no_tags),
            link: '/triage?has_tags=false',
            hint: 'Tag them in triage mode',
        },
        {
            key: 'actors',
            label: 'Without actors',
            count: p.no_actors,
            icon: 'heroicons:user',
            share: share(p.no_actors),
            link: '/triage?has_actors=false',
            hint: 'Add actors in triage mode',
        },
        {
            key: 'studio',
            label: 'Without studio',
            count: p.no_studio,
            icon: 'heroicons:building-office-2',
            share: share(p.no_studio),
            link: '/triage?has_studio=false',
            hint: 'Set studios in triage mode',
        },
        {
            key: 'rating',
            label: 'Watched, not rated',
            count: p.watched_not_rated,
            icon: 'heroicons:star',
            link: '/triage?watched=true&rated=false',
            hint: 'Rate the scenes you watched, in triage mode',
        },
        {
            key: 'thumbnail',
            label: 'Without thumbnail',
            count: p.no_thumbnail,
            icon: 'heroicons:photo',
            share: share(p.no_thumbnail),
            link: p.is_admin ? '/settings?tab=jobs' : undefined,
        },
        {
            key: 'sprites',
            label: 'Without sprites',
            count: p.no_sprites,
            icon: 'heroicons:film',
            share: share(p.no_sprites),
            link: p.is_admin ? '/settings?tab=jobs' : undefined,
        },
    ];
    if (p.is_admin && p.unresolved_duplicates !== undefined) {
        list.push({
            key: 'duplicates',
            label: 'Duplicate groups to review',
            count: p.unresolved_duplicates,
            icon: 'heroicons:document-duplicate',
            link: '/duplicates',
        });
    }
    return list;
});

const allDone = computed(() => items.value.length > 0 && items.value.every((i) => i.count === 0));
</script>

<template>
    <div class="border-border rounded-lg border bg-white/2 p-4">
        <div v-if="allDone" class="text-emerald flex items-center gap-2 py-4 text-xs">
            <Icon name="heroicons:check-circle" size="16" />
            Everything is organized.
        </div>
        <div v-else class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4">
            <component
                :is="item.link ? NuxtLinkComponent : 'div'"
                v-for="item in items"
                :key="item.key"
                :to="item.link"
                :title="item.hint"
                class="border-border flex items-center gap-3 rounded-md border px-3 py-2.5
                    transition-all"
                :class="[
                    item.link ? 'hover:border-lava/40 hover:bg-white/4' : '',
                    item.count === 0 ? 'opacity-50' : '',
                ]"
            >
                <Icon
                    :name="item.icon"
                    size="16"
                    class="shrink-0"
                    :class="item.count > 0 ? 'text-lava' : 'text-emerald'"
                />
                <div class="min-w-0 flex-1">
                    <div class="text-dim truncate text-[11px]">{{ item.label }}</div>
                    <div class="flex items-baseline gap-1.5">
                        <span class="font-mono text-sm font-semibold text-white">{{
                            number.format(item.count)
                        }}</span>
                        <span
                            v-if="item.share !== undefined && item.count > 0"
                            class="text-dim font-mono text-[10px]"
                            >{{ Math.round(item.share * 100) }}%</span
                        >
                    </div>
                    <div
                        v-if="item.share !== undefined && item.count > 0"
                        class="mt-1 h-0.5 rounded-full bg-white/10"
                    >
                        <div
                            class="bg-lava/70 h-full rounded-full"
                            :style="{ width: `${Math.max(2, item.share * 100)}%` }"
                        />
                    </div>
                </div>
                <Icon
                    v-if="item.link"
                    name="heroicons:chevron-right"
                    size="12"
                    class="text-dim shrink-0"
                />
            </component>
        </div>
    </div>
</template>
