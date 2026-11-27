<script setup lang="ts">
// "Start at" of a scene share: a checkbox and a time (seconds, m:ss or h:mm:ss), checked
// against the scene duration when known.
const props = defineProps<{ max?: number }>();
const enabled = defineModel<boolean>('enabled', { required: true });
const seconds = defineModel<number>('seconds', { required: true });

const text = ref(formatClock(seconds.value));
const invalid = ref(false);

watch(seconds, (v) => {
    if (parseClockInput(text.value) !== v) text.value = formatClock(v);
});

const commit = () => {
    const v = parseClockInput(text.value);
    invalid.value = v === null || (!!props.max && v >= props.max);
    if (v !== null && !invalid.value) {
        seconds.value = v;
        if (v > 0) enabled.value = true;
    }
};
</script>

<template>
    <div class="flex items-center gap-2">
        <label class="flex cursor-pointer items-center gap-2 text-xs text-white/85">
            <input v-model="enabled" type="checkbox" class="accent-lava h-3.5 w-3.5" />
            Start at
        </label>
        <input
            v-model="text"
            type="text"
            inputmode="numeric"
            class="bg-surface focus:border-lava/40 w-24 rounded-md border px-2 py-1 font-mono
                text-xs text-white outline-none"
            :class="invalid ? 'border-lava/60' : 'border-border'"
            aria-label="Start time"
            placeholder="0:00"
            @input="invalid = false"
            @blur="commit"
            @keydown.enter.prevent="commit"
        />
        <span v-if="invalid" class="text-lava text-[10px]">
            {{ max ? `Between 0:00 and ${formatClock(max - 1)}` : 'Use m:ss or h:mm:ss' }}
        </span>
    </div>
</template>
