<script setup lang="ts">
// Ordered criteria used to pick the best variant of a duplicate group (first difference wins).
const props = defineProps<{
    modelValue: string[] | undefined;
    available?: string[];
}>();

const emit = defineEmits<{
    'update:modelValue': [value: string[]];
}>();

const DEFAULT_CRITERIA = ['resolution', 'duration', 'bitrate', 'size_small'];
const ALL_CRITERIA = [
    'resolution',
    'duration',
    'bitrate',
    'size_small',
    'size_large',
    'frame_rate',
    'codec',
    'oldest',
    'newest',
];

const LABELS: Record<string, { label: string; desc: string }> = {
    resolution: { label: 'Resolution', desc: 'Highest resolution first' },
    duration: { label: 'Duration', desc: 'Longest first' },
    bitrate: { label: 'Bitrate', desc: 'Highest bitrate first' },
    size_small: { label: 'Smallest file', desc: 'Smallest file first' },
    size_large: { label: 'Largest file', desc: 'Largest file first' },
    frame_rate: { label: 'Frame rate', desc: 'Highest frame rate first' },
    codec: { label: 'Codec', desc: 'AV1 > HEVC > H.264 > other' },
    oldest: { label: 'Oldest', desc: 'Added to the library first' },
    newest: { label: 'Newest', desc: 'Added to the library last' },
};

// Pairs that cannot be used together
const EXCLUSIVE: Record<string, string> = {
    size_small: 'size_large',
    size_large: 'size_small',
    oldest: 'newest',
    newest: 'oldest',
};

const selected = computed(() => (props.modelValue?.length ? props.modelValue : DEFAULT_CRITERIA));

const unused = computed(() =>
    (props.available?.length ? props.available : ALL_CRITERIA).filter(
        (c) => !selected.value.includes(c) && !selected.value.includes(EXCLUSIVE[c] ?? ''),
    ),
);

const update = (value: string[]) => emit('update:modelValue', value);

const move = (index: number, offset: number) => {
    const next = [...selected.value];
    const target = index + offset;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target]!, next[index]!];
    update(next);
};

const remove = (index: number) => {
    if (selected.value.length <= 1) return;
    update(selected.value.filter((_, i) => i !== index));
};

const add = (criterion: string) => update([...selected.value, criterion]);

const reset = () => update([...DEFAULT_CRITERIA]);
</script>

<template>
    <div class="border-border mt-2 border-t pt-4">
        <div class="mb-1 flex items-center justify-between">
            <h3 class="text-sm font-medium text-white">Best Variant Selection</h3>
            <button class="text-dim text-[10px] transition-colors hover:text-white" @click="reset">
                Reset to default
            </button>
        </div>
        <p class="text-dim mb-3 text-[10px]">
            Used by "Auto-select best" and automatic resolution. Criteria are applied in order: the
            first one that differs between two scenes decides.
        </p>

        <ol class="space-y-1">
            <li
                v-for="(criterion, index) in selected"
                :key="criterion"
                class="border-border flex items-center gap-2 rounded-md border bg-white/2 px-2.5
                    py-1.5"
            >
                <span class="text-lava w-4 font-mono text-[10px]">{{ index + 1 }}</span>
                <div class="min-w-0 flex-1">
                    <div class="text-xs text-white">
                        {{ LABELS[criterion]?.label ?? criterion }}
                    </div>
                    <div class="text-dim text-[10px]">{{ LABELS[criterion]?.desc }}</div>
                </div>
                <button
                    class="text-dim p-0.5 transition-colors hover:text-white disabled:opacity-20"
                    :disabled="index === 0"
                    title="Move up"
                    @click="move(index, -1)"
                >
                    <Icon name="heroicons:chevron-up" size="12" />
                </button>
                <button
                    class="text-dim p-0.5 transition-colors hover:text-white disabled:opacity-20"
                    :disabled="index === selected.length - 1"
                    title="Move down"
                    @click="move(index, 1)"
                >
                    <Icon name="heroicons:chevron-down" size="12" />
                </button>
                <button
                    class="text-dim hover:text-lava p-0.5 transition-colors disabled:opacity-20"
                    :disabled="selected.length <= 1"
                    title="Remove"
                    @click="remove(index)"
                >
                    <Icon name="heroicons:x-mark" size="12" />
                </button>
            </li>
        </ol>

        <div v-if="unused.length" class="mt-2 flex flex-wrap gap-1">
            <button
                v-for="criterion in unused"
                :key="criterion"
                class="border-border text-dim hover:border-lava/30 hover:text-lava flex items-center
                    gap-1 rounded-md border px-2 py-0.5 text-[10px] transition-all"
                :title="LABELS[criterion]?.desc"
                @click="add(criterion)"
            >
                <Icon name="heroicons:plus" size="10" />
                {{ LABELS[criterion]?.label ?? criterion }}
            </button>
        </div>
    </div>
</template>
