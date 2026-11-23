<script setup lang="ts">
import type { SceneCopyFilters } from '~/types/duplicates';
import type { StoragePath } from '~/types/storage';

// Filters and sort of the copy list. Text inputs are applied on Enter or blur.
const props = defineProps<{
    modelValue: SceneCopyFilters;
    storagePaths: StoragePath[];
}>();

const emit = defineEmits<{
    'update:modelValue': [filters: SceneCopyFilters];
}>();

const search = ref(props.modelValue.search);
const minSize = ref(props.modelValue.minSizeMB?.toString() ?? '');
const maxSize = ref(props.modelValue.maxSizeMB?.toString() ?? '');

watch(
    () => props.modelValue,
    (f) => {
        search.value = f.search;
        minSize.value = f.minSizeMB?.toString() ?? '';
        maxSize.value = f.maxSizeMB?.toString() ?? '';
    },
);

const update = (patch: Partial<SceneCopyFilters>) => {
    emit('update:modelValue', { ...props.modelValue, ...patch });
};

const applyText = () => {
    const next = {
        search: search.value.trim(),
        minSizeMB: parseSizeMB(minSize.value),
        maxSizeMB: parseSizeMB(maxSize.value),
    };
    const cur = props.modelValue;
    if (
        next.search !== cur.search ||
        next.minSizeMB !== cur.minSizeMB ||
        next.maxSizeMB !== cur.maxSizeMB
    ) {
        update(next);
    }
};

const onStoragePath = (raw: string) => {
    update({ storagePathId: raw ? Number(raw) : null });
};
</script>

<template>
    <div class="mb-3 flex flex-wrap items-center gap-2">
        <div class="relative min-w-48 flex-1">
            <Icon
                name="heroicons:magnifying-glass"
                size="12"
                class="text-dim absolute top-1/2 left-2 -translate-y-1/2"
            />
            <input
                v-model="search"
                type="search"
                placeholder="Search a path or title"
                class="border-border bg-panel w-full rounded-md border py-1 pr-2 pl-6 text-[11px]
                    text-white placeholder:text-white/30 focus:border-white/30 focus:outline-none"
                @keydown.enter="applyText"
                @blur="applyText"
            />
        </div>
        <select
            :value="modelValue.storagePathId ?? ''"
            class="border-border bg-panel rounded-md border px-2 py-1 text-[11px] text-white"
            title="Storage path of the copies"
            @change="onStoragePath(($event.target as HTMLSelectElement).value)"
        >
            <option value="">All storage paths</option>
            <option v-for="sp in storagePaths" :key="sp.id" :value="sp.id">
                {{ sp.name }}{{ sp.read_only ? ' (read-only)' : '' }}
            </option>
        </select>
        <div class="flex items-center gap-1 text-[11px]">
            <input
                v-model="minSize"
                inputmode="decimal"
                placeholder="Min MB"
                class="border-border bg-panel w-16 rounded-md border px-2 py-1 text-white
                    placeholder:text-white/30 focus:border-white/30 focus:outline-none"
                @keydown.enter="applyText"
                @blur="applyText"
            />
            <span class="text-dim">-</span>
            <input
                v-model="maxSize"
                inputmode="decimal"
                placeholder="Max MB"
                class="border-border bg-panel w-16 rounded-md border px-2 py-1 text-white
                    placeholder:text-white/30 focus:border-white/30 focus:outline-none"
                @keydown.enter="applyText"
                @blur="applyText"
            />
        </div>
        <select
            :value="modelValue.sort"
            class="border-border bg-panel rounded-md border px-2 py-1 text-[11px] text-white"
            @change="
                update({
                    sort: ($event.target as HTMLSelectElement).value as SceneCopyFilters['sort'],
                })
            "
        >
            <option v-for="s in COPY_SORTS" :key="s.value" :value="s.value">{{ s.label }}</option>
        </select>
    </div>
</template>
