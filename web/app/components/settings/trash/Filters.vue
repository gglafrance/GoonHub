<script setup lang="ts">
// Filters panel of the trash list. Every change is applied right away (and lands in the URL).
import type { TrashFacets } from '~/types/trash';
import type { TrashFilterState } from '~/utils/trashFilters';

const props = defineProps<{
    filters: TrashFilterState;
    facets: TrashFacets | null;
}>();

const emit = defineEmits<{ 'update:filters': [value: TrashFilterState] }>();

const { formatSize } = useFormatter();

const set = <K extends keyof TrashFilterState>(key: K, value: TrashFilterState[K]) => {
    emit('update:filters', { ...props.filters, [key]: value });
};

const numberFrom = (event: Event) => {
    const n = Number((event.target as HTMLInputElement).value);
    return Number.isFinite(n) && n > 0 ? n : 0;
};
const textFrom = (event: Event) => (event.target as HTMLInputElement | HTMLSelectElement).value;

// Size inputs in MB or GB (the filter is kept in MB)
const sizeUnit = ref<'MB' | 'GB'>('GB');
const sizeFactor = computed(() => (sizeUnit.value === 'GB' ? 1024 : 1));
const sizeInput = (mb: number) => (mb ? +(mb / sizeFactor.value).toFixed(2) : '');
const setSize = (key: 'minSizeMb' | 'maxSizeMb', event: Event) =>
    set(key, numberFrom(event) * sizeFactor.value);

const storageOptions = computed(() =>
    (props.facets?.storage_paths ?? []).map((p) => ({
        value: p.storage_path_id === null ? 'none' : String(p.storage_path_id),
        label: `${p.name ?? 'No storage path'} (${p.count}, ${formatSize(p.size)})`,
    })),
);

const reset = () => emit('update:filters', { ...emptyTrashFilters(), q: props.filters.q });

const inputClass = `border-border bg-surface w-full rounded-md border px-2 py-1.5 text-xs
    text-white focus:border-white/20 focus:outline-none`;
const labelClass = 'text-dim mb-1 block text-[10px] font-medium tracking-wide uppercase';
</script>

<template>
    <div
        class="border-border grid gap-3 rounded-lg border bg-white/2 p-3 sm:grid-cols-2
            lg:grid-cols-4"
    >
        <label>
            <span :class="labelClass">Storage path</span>
            <select
                :value="filters.storage"
                :class="inputClass"
                @change="set('storage', textFrom($event))"
            >
                <option value="">Any</option>
                <option v-for="o in storageOptions" :key="o.value" :value="o.value">
                    {{ o.label }}
                </option>
            </select>
        </label>
        <label>
            <span :class="labelClass">Resolution</span>
            <select
                :value="filters.resolution"
                :class="inputClass"
                @change="set('resolution', textFrom($event))"
            >
                <option value="">Any</option>
                <option v-for="r in TRASH_RESOLUTIONS" :key="r.value" :value="r.value">
                    {{ r.label }}
                </option>
            </select>
        </label>
        <label>
            <span :class="labelClass">Codec</span>
            <select
                :value="filters.codec"
                :class="inputClass"
                @change="set('codec', textFrom($event))"
            >
                <option value="">Any</option>
                <option
                    v-for="c in facets?.codecs ?? []"
                    :key="c.codec"
                    :value="c.codec"
                    :disabled="!c.codec"
                >
                    {{ c.codec || 'Unknown' }} ({{ c.count }})
                </option>
            </select>
        </label>
        <label>
            <span :class="labelClass">Origin</span>
            <select
                :value="filters.source"
                :class="inputClass"
                @change="set('source', textFrom($event) as TrashFilterState['source'])"
            >
                <option value="">Any</option>
                <option value="duplicates">
                    Duplicate resolution{{ facets ? ` (${facets.duplicates})` : '' }}
                </option>
                <option value="other">Other (manual, explorer...)</option>
            </select>
        </label>

        <div>
            <span :class="labelClass">Size ({{ sizeUnit }})</span>
            <div class="flex items-center gap-1.5">
                <input
                    :value="sizeInput(filters.minSizeMb)"
                    type="number"
                    min="0"
                    step="any"
                    placeholder="Min"
                    :class="inputClass"
                    aria-label="Minimum size"
                    @change="setSize('minSizeMb', $event)"
                />
                <span class="text-dim text-[10px]">-</span>
                <input
                    :value="sizeInput(filters.maxSizeMb)"
                    type="number"
                    min="0"
                    step="any"
                    placeholder="Max"
                    :class="inputClass"
                    aria-label="Maximum size"
                    @change="setSize('maxSizeMb', $event)"
                />
                <button
                    class="border-border text-dim shrink-0 rounded-md border px-1.5 py-1 text-[10px]
                        transition-colors hover:text-white"
                    title="Switch unit"
                    @click="sizeUnit = sizeUnit === 'GB' ? 'MB' : 'GB'"
                >
                    {{ sizeUnit }}
                </button>
            </div>
        </div>
        <div>
            <span :class="labelClass">Duration (minutes)</span>
            <div class="flex items-center gap-1.5">
                <input
                    :value="filters.minDurationMin || ''"
                    type="number"
                    min="0"
                    step="any"
                    placeholder="Min"
                    :class="inputClass"
                    aria-label="Minimum duration"
                    @change="set('minDurationMin', numberFrom($event))"
                />
                <span class="text-dim text-[10px]">-</span>
                <input
                    :value="filters.maxDurationMin || ''"
                    type="number"
                    min="0"
                    step="any"
                    placeholder="Max"
                    :class="inputClass"
                    aria-label="Maximum duration"
                    @change="set('maxDurationMin', numberFrom($event))"
                />
            </div>
        </div>
        <div class="sm:col-span-2">
            <span :class="labelClass">Trashed between</span>
            <div class="flex items-center gap-1.5">
                <input
                    :value="filters.from"
                    type="date"
                    :class="inputClass"
                    aria-label="Trashed from"
                    @change="set('from', textFrom($event))"
                />
                <span class="text-dim text-[10px]">-</span>
                <input
                    :value="filters.to"
                    type="date"
                    :class="inputClass"
                    aria-label="Trashed to"
                    @change="set('to', textFrom($event))"
                />
                <button
                    class="text-dim hover:text-lava shrink-0 px-1.5 text-[11px] transition-colors"
                    @click="reset"
                >
                    Reset
                </button>
            </div>
        </div>
    </div>
</template>
