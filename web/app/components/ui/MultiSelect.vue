<script setup lang="ts">
const props = defineProps<{
    modelValue: string[];
    options: { value: string; label: string }[];
    placeholder?: string;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: string[]];
}>();

const open = ref(false);
const triggerRef = ref<HTMLElement>();

const selectedCount = computed(() => props.modelValue.length);

const label = computed(() => {
    if (selectedCount.value === 0) return props.placeholder || 'All';
    if (selectedCount.value === 1) {
        const opt = props.options.find((o) => o.value === props.modelValue[0]);
        return opt?.label || props.modelValue[0];
    }
    return `${selectedCount.value} selected`;
});

const toggle = (value: string) => {
    const current = [...props.modelValue];
    const idx = current.indexOf(value);
    if (idx >= 0) {
        current.splice(idx, 1);
    } else {
        current.push(value);
    }
    emit('update:modelValue', current);
};

const isSelected = (value: string) => props.modelValue.includes(value);

const handleClickOutside = (e: MouseEvent) => {
    if (triggerRef.value && !triggerRef.value.contains(e.target as Node)) {
        open.value = false;
    }
};

onMounted(() => document.addEventListener('click', handleClickOutside));
onUnmounted(() => document.removeEventListener('click', handleClickOutside));
</script>

<template>
    <div ref="triggerRef" class="relative">
        <button
            type="button"
            class="border-border bg-panel focus:border-lava/50 focus:ring-lava/20 flex h-full
                cursor-pointer items-center gap-1.5 rounded-lg border py-2 pr-7 pl-3 text-xs
                text-white transition-all focus:ring-2 focus:outline-none"
            @click="open = !open"
        >
            <span class="whitespace-nowrap">{{ label }}</span>
            <span
                v-if="selectedCount > 0"
                class="bg-lava/90 inline-flex h-4 min-w-4 items-center justify-center rounded-full
                    px-1 text-[10px] leading-none font-bold text-white"
            >
                {{ selectedCount }}
            </span>
        </button>
        <Icon
            name="heroicons:chevron-down"
            size="14"
            class="text-dim pointer-events-none absolute top-1/2 right-2 -translate-y-1/2
                transition-transform"
            :class="{ 'rotate-180': open }"
        />

        <Transition
            enter-active-class="transition duration-100 ease-out"
            enter-from-class="scale-95 opacity-0"
            enter-to-class="scale-100 opacity-100"
            leave-active-class="transition duration-75 ease-in"
            leave-from-class="scale-100 opacity-100"
            leave-to-class="scale-95 opacity-0"
        >
            <div
                v-if="open"
                class="border-border bg-panel absolute top-full right-0 z-50 mt-1 min-w-full
                    overflow-hidden rounded-lg border shadow-lg"
            >
                <button
                    v-for="opt in options"
                    :key="opt.value"
                    type="button"
                    class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs
                        transition-colors hover:bg-white/5"
                    @click.stop="toggle(opt.value)"
                >
                    <span
                        class="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded border
                            transition-colors"
                        :class="
                            isSelected(opt.value)
                                ? 'bg-lava border-lava'
                                : 'border-white/20 bg-transparent'
                        "
                    >
                        <Icon
                            v-if="isSelected(opt.value)"
                            name="heroicons:check"
                            size="10"
                            class="text-white"
                        />
                    </span>
                    <span
                        class="whitespace-nowrap"
                        :class="isSelected(opt.value) ? 'text-white' : 'text-white/70'"
                    >
                        {{ opt.label }}
                    </span>
                </button>

                <button
                    v-if="selectedCount > 0"
                    type="button"
                    class="text-dim w-full border-t border-white/5 px-3 py-1.5 text-left text-[11px]
                        transition-colors hover:text-white"
                    @click.stop="emit('update:modelValue', [])"
                >
                    Clear all
                </button>
            </div>
        </Transition>
    </div>
</template>
