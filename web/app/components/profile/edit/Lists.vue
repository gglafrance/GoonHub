<script setup lang="ts">
import type { CustomList, CustomListInput, ProfileEdit } from '~/types/profile';
import type { Audience } from '~/types/social';

// Custom lists editor: the owner's lists (title, criteria summary, audience, what they depend
// on), create from scratch or from a preset, edit, delete. A list is placed on the profile with
// a "Custom list" block (Tabs & layout).
const props = defineProps<{ state: ProfileEdit }>();
const emit = defineEmits<{ saved: [] }>();

const api = useApiProfiles();
const lists = ref<CustomList[]>([]);
const error = ref('');
const editing = ref<{ id: number | null; input: CustomListInput } | null>(null);
const deleting = ref<CustomList | null>(null);

watch(
    () => props.state.lists,
    (v) => (lists.value = v.slice()),
    { immediate: true },
);

const SORT_LABELS = computed(() =>
    Object.fromEntries([
        ['jizz_count_desc', 'Most O'],
        ['last_jizz_desc', 'Last O'],
        ['view_count_desc', 'Most viewed'],
        ['created_at_desc', 'Newest'],
        ['random', 'Random'],
        ...USER_SORT_OPTIONS.map((o) => [o.value, o.label]),
    ] as [string, string][]),
);
const audienceLabel = (a: Audience) =>
    props.state.privacy.audiences.find((x) => x.value === a)?.label ?? a;

function startNew(preset?: (typeof CUSTOM_LIST_PRESETS)[number]) {
    editing.value = {
        id: null,
        input: {
            title: preset?.title ?? '',
            filters: { ...(preset?.filters ?? {}) },
            audience: 'public',
            figures: 'auto',
        },
    };
}

/** The figures a list shows (auto: what the server computed on its effective criteria). */
const figuresOf = (l: CustomList) =>
    l.figures === 'auto' ? (l.auto_figures ?? []) : listFigures(l.figures, l.filters);

function startEdit(l: CustomList) {
    editing.value = {
        id: l.id,
        input: {
            title: l.title,
            filters: { ...l.filters },
            saved_search_uuid: l.follows_saved_search
                ? (l.saved_search_uuid ?? undefined)
                : undefined,
            audience: l.audience,
            figures: l.figures,
        },
    };
}

function onSaved(list: CustomList) {
    const i = lists.value.findIndex((l) => l.id === list.id);
    if (i >= 0) lists.value[i] = list;
    else lists.value.push(list);
    editing.value = null;
    emit('saved');
}

async function remove() {
    const l = deleting.value;
    deleting.value = null;
    if (!l) return;
    error.value = '';
    try {
        await api.deleteList(l.id);
        lists.value = lists.value.filter((x) => x.id !== l.id);
        emit('saved');
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to delete the list';
    }
}
</script>

<template>
    <div class="space-y-3">
        <div v-if="lists.length === 0" class="text-dim text-xs">
            No custom list yet. A list is a search (filters and sort) run on your library with your
            own ratings, likes, O and views; put it on your profile with a "Custom list" block.
        </div>
        <div v-else class="border-border divide-border divide-y rounded-lg border">
            <div v-for="l in lists" :key="l.id" class="flex flex-wrap items-center gap-2 px-3 py-2">
                <Icon name="heroicons:queue-list" size="14" class="text-lava" />
                <div class="min-w-0 flex-1">
                    <div class="text-xs font-medium text-white">{{ l.title }}</div>
                    <div class="text-dim truncate text-[10px]">
                        <span v-if="l.follows_saved_search">Saved search · </span>
                        {{ criteriaSummary(l.filters, SORT_LABELS) }}
                    </div>
                    <div class="text-dim mt-0.5 flex flex-wrap gap-1 text-[10px]">
                        <span class="rounded bg-white/5 px-1.5">
                            List: {{ audienceLabel(l.audience) }}
                        </span>
                        <span
                            v-for="d in l.dependencies"
                            :key="d.key"
                            class="rounded bg-white/5 px-1.5"
                            :title="`The list is only shown to who can also see your ${d.label}`"
                        >
                            {{ d.label }}: {{ audienceLabel(d.audience) }}
                        </span>
                        <span
                            v-if="figuresOf(l).length"
                            class="rounded bg-white/5 px-1.5"
                            title="Your figures shown under the cards"
                        >
                            Figures:
                            {{
                                figuresOf(l)
                                    .map((k) => LIST_FIGURE_LABELS[k])
                                    .join(', ')
                            }}
                        </span>
                        <span v-if="l.needs_size" class="rounded bg-white/5 px-1.5">
                            file sizes: viewers with the permission only
                        </span>
                    </div>
                </div>
                <button class="text-dim hover:text-white" title="Edit" @click="startEdit(l)">
                    <Icon name="heroicons:pencil-square" size="14" />
                </button>
                <button class="text-dim hover:text-lava" title="Delete" @click="deleting = l">
                    <Icon name="heroicons:trash" size="14" />
                </button>
            </div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
            <button
                class="bg-lava hover:bg-lava/90 flex items-center gap-1 rounded-md px-3 py-1.5
                    text-xs font-medium text-white disabled:opacity-50"
                :disabled="lists.length >= props.state.limits.max_lists"
                @click="startNew()"
            >
                <Icon name="heroicons:plus" size="12" />
                New list
            </button>
            <span class="text-dim text-[10px]">or start from</span>
            <button
                v-for="p in CUSTOM_LIST_PRESETS"
                :key="p.title"
                class="border-border text-dim hover:border-lava/40 hover:text-lava rounded-full
                    border px-2.5 py-0.5 text-[11px] transition-colors disabled:opacity-40"
                :disabled="lists.length >= props.state.limits.max_lists"
                @click="startNew(p)"
            >
                {{ p.title }}
            </button>
            <span class="text-dim ml-auto font-mono text-[10px]">
                {{ lists.length }}/{{ props.state.limits.max_lists }}
            </span>
        </div>
        <p v-if="error" class="text-lava text-[11px]">{{ error }}</p>
        <ProfileEditListForm
            v-if="editing"
            :list-id="editing.id"
            :initial="editing.input"
            :privacy="props.state.privacy"
            @close="editing = null"
            @saved="onSaved"
        />
        <UiConfirmModal
            :visible="!!deleting"
            title="Delete this list?"
            confirm-label="Delete"
            @close="deleting = null"
            @confirm="remove"
        >
            <p class="text-dim text-xs">
                “{{ deleting?.title }}” is deleted; its blocks then show nothing until you remove
                them.
            </p>
        </UiConfirmModal>
    </div>
</template>
