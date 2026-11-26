<script setup lang="ts">
import type { CustomList, CustomListInput } from '~/types/profile';
import type { SavedSearch } from '~/types/saved_search';
import type { Audience, PrivacySettings } from '~/types/social';

// Create or edit a custom list (modal): title, who can see it, and its source: criteria defined
// with the search interface, or one of the owner's saved searches (followed live). Shows which
// privacy items the criteria depend on and their current audience, and the owner's figures
// shown under the cards (ProfileEditListFigures).
const props = defineProps<{
    listId: number | null;
    initial: CustomListInput;
    privacy: PrivacySettings;
}>();
const emit = defineEmits<{ close: []; saved: [list: CustomList] }>();

const api = useApiProfiles();
const savedApi = useApiSavedSearches();
const form = ref<CustomListInput>({
    ...props.initial,
    filters: { ...props.initial.filters },
    figures: props.initial.figures ?? 'auto',
});
const source = ref<'criteria' | 'saved'>(props.initial.saved_search_uuid ? 'saved' : 'criteria');
const savedSearches = ref<SavedSearch[]>([]);
const showCriteria = ref(false);
const saving = ref(false);
const error = ref('');

onMounted(async () => {
    try {
        savedSearches.value = (await savedApi.fetchSavedSearches()).data ?? [];
    } catch {
        savedSearches.value = [];
    }
});

const effectiveFilters = computed(() => {
    if (source.value === 'saved') {
        return (
            savedSearches.value.find((s) => s.uuid === form.value.saved_search_uuid)?.filters ?? {}
        );
    }
    return form.value.filters;
});

const deps = computed(() => {
    const d = criteriaDependencies(effectiveFilters.value);
    return {
        size: d.size,
        items: d.items.map((key) => {
            const item = props.privacy.items.find((i) => i.key === key);
            return { key, label: item?.label ?? key, audience: item?.audience };
        }),
    };
});
const audienceLabel = (a?: Audience) =>
    props.privacy.audiences.find((x) => x.value === a)?.label ?? a ?? '';
const audienceOptions = computed(() =>
    props.privacy.audiences.map((a) => ({ value: a.value, label: a.label })),
);
const savedOptions = computed(() => [
    { value: '', label: 'Pick a saved search...' },
    ...savedSearches.value.map((s) => ({ value: s.uuid, label: s.name })),
]);
const summary = computed(() =>
    criteriaSummary(
        effectiveFilters.value,
        Object.fromEntries(USER_SORT_OPTIONS.map((o) => [o.value, o.label])),
    ),
);

async function save() {
    saving.value = true;
    error.value = '';
    const input: CustomListInput = {
        title: form.value.title,
        audience: form.value.audience,
        filters: source.value === 'criteria' ? form.value.filters : {},
        saved_search_uuid: source.value === 'saved' ? form.value.saved_search_uuid : undefined,
        figures: form.value.figures ?? 'auto',
    };
    try {
        const list =
            props.listId === null
                ? await api.createList(input)
                : await api.updateList(props.listId, input);
        emit('saved', list);
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save the list';
    } finally {
        saving.value = false;
    }
}

const onKey = (e: KeyboardEvent) => {
    if (e.key === 'Escape' && !showCriteria.value) emit('close');
};
onMounted(() => window.addEventListener('keydown', onKey));
onUnmounted(() => window.removeEventListener('keydown', onKey));
</script>

<template>
    <Teleport to="body">
        <div
            class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4
                backdrop-blur-sm"
            @click.self="emit('close')"
        >
            <div class="glass-panel w-full max-w-lg space-y-3 p-5" role="dialog" aria-modal="true">
                <h3 class="text-sm font-semibold text-white">
                    {{ props.listId === null ? 'New custom list' : 'Edit the list' }}
                </h3>
                <label class="text-dim block text-[11px]">
                    Title
                    <input
                        v-model="form.title"
                        maxlength="80"
                        class="border-border mt-1 w-full rounded-lg border bg-white/[0.03] px-3 py-2
                            text-xs text-white"
                        placeholder="Top O videos"
                    />
                </label>
                <label class="text-dim block text-[11px]">
                    Who can see the list
                    <UiSelectMenu
                        :model-value="form.audience"
                        :options="audienceOptions"
                        @update:model-value="form.audience = String($event) as Audience"
                    />
                </label>
                <div class="flex gap-1">
                    <button
                        v-for="s in [
                            { id: 'criteria' as const, label: 'Criteria' },
                            { id: 'saved' as const, label: 'A saved search' },
                        ]"
                        :key="s.id"
                        class="rounded-md px-2.5 py-1 text-[11px] font-medium"
                        :class="
                            source === s.id ? 'bg-lava/10 text-lava' : 'text-dim hover:text-white'
                        "
                        @click="source = s.id"
                    >
                        {{ s.label }}
                    </button>
                </div>
                <div v-if="source === 'criteria'" class="flex items-center gap-2">
                    <p class="text-dim min-w-0 flex-1 truncate text-[11px]">{{ summary }}</p>
                    <button
                        class="border-border hover:border-lava/40 hover:text-lava rounded-md border
                            px-2.5 py-1 text-[11px] text-white"
                        @click="showCriteria = true"
                    >
                        Define criteria
                    </button>
                </div>
                <div v-else class="space-y-1">
                    <UiSelectMenu
                        :model-value="form.saved_search_uuid ?? ''"
                        :options="savedOptions"
                        @update:model-value="form.saved_search_uuid = String($event) || undefined"
                    />
                    <p class="text-dim text-[10px]">
                        The list follows the saved search: changing it changes the list.
                    </p>
                </div>
                <div class="border-border rounded-lg border px-3 py-2 text-[11px]">
                    <div class="text-dim mb-1">Shown only to who can also see:</div>
                    <div v-if="deps.items.length === 0 && !deps.size" class="text-dim">
                        Nothing more (the criteria use no personal data).
                    </div>
                    <div v-for="d in deps.items" :key="d.key" class="text-white">
                        {{ d.label }}
                        <span class="text-dim">({{ audienceLabel(d.audience) }})</span>
                    </div>
                    <div v-if="deps.size" class="text-white">
                        File sizes <span class="text-dim">(viewers with the permission)</span>
                    </div>
                </div>
                <ProfileEditListFigures
                    :model-value="form.figures ?? 'auto'"
                    :filters="effectiveFilters"
                    :privacy="props.privacy"
                    @update:model-value="form.figures = $event"
                />
                <p v-if="error" class="text-lava text-[11px]">{{ error }}</p>
                <div class="flex justify-end gap-2">
                    <button class="text-dim text-xs hover:text-white" @click="emit('close')">
                        Cancel
                    </button>
                    <button
                        class="bg-lava hover:bg-lava/90 rounded-md px-3 py-1.5 text-xs font-medium
                            text-white disabled:opacity-50"
                        :disabled="
                            saving ||
                            !form.title.trim() ||
                            (source === 'saved' && !form.saved_search_uuid)
                        "
                        @click="save"
                    >
                        {{ saving ? 'Saving...' : 'Save' }}
                    </button>
                </div>
            </div>
        </div>
        <ProfileEditListCriteria
            v-if="showCriteria"
            :filters="form.filters"
            @close="showCriteria = false"
            @apply="
                (f) => {
                    form.filters = f;
                    showCriteria = false;
                }
            "
        />
    </Teleport>
</template>
