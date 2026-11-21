<script setup lang="ts">
// Searchable multi (or single) choice list for the marker page filters.
interface PickItem {
    value: string;
    label: string;
    count?: number;
    color?: string;
}

const props = defineProps<{
    title: string;
    icon: string;
    items: PickItem[];
    modelValue: string[];
    single?: boolean;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: string[]];
}>();

const collapsed = ref(true);
const query = ref('');

const filtered = computed(() => {
    if (!query.value) return props.items;
    const q = query.value.toLowerCase();
    return props.items.filter((i) => i.label.toLowerCase().includes(q));
});

const toggle = (value: string) => {
    const selected = props.modelValue.includes(value);
    if (props.single) {
        emit('update:modelValue', selected ? [] : [value]);
        return;
    }
    emit(
        'update:modelValue',
        selected ? props.modelValue.filter((v) => v !== value) : [...props.modelValue, value],
    );
};

const badge = computed(() => {
    if (props.modelValue.length === 0) return undefined;
    return props.single ? props.modelValue[0] : props.modelValue.length;
});
</script>

<template>
    <SearchFiltersFilterSection
        :title="title"
        :icon="icon"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <input
            v-model="query"
            type="text"
            :placeholder="`Search ${title.toLowerCase()}...`"
            class="border-border bg-surface text-dim mb-2 w-full rounded-md border px-2 py-1.5
                text-xs placeholder-white/30 focus:border-white/20 focus:outline-none"
        />
        <div v-if="filtered.length === 0" class="text-dim px-2 py-1 text-xs">No matches</div>
        <div v-else class="max-h-40 space-y-1 overflow-y-auto">
            <button
                v-for="item in filtered"
                :key="item.value"
                class="flex w-full items-center gap-2 rounded px-2 py-1 text-left text-xs
                    transition-colors"
                :class="
                    modelValue.includes(item.value)
                        ? 'bg-lava/10 text-white'
                        : 'text-dim hover:bg-white/5 hover:text-white'
                "
                @click="toggle(item.value)"
            >
                <span
                    v-if="item.color"
                    class="h-2 w-2 shrink-0 rounded-full"
                    :style="{ backgroundColor: item.color }"
                />
                <span class="flex-1 truncate">{{ item.label }}</span>
                <span v-if="item.count !== undefined" class="text-[10px] opacity-50">
                    {{ item.count }}
                </span>
            </button>
        </div>
    </SearchFiltersFilterSection>
</template>
