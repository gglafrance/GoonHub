<script setup lang="ts">
import type { Audience, PrivacyItem, PrivacyOption } from '~/types/social';

// One privacy item: label, description and its audience selector ("(default)" marks the
// catalog default).
const props = defineProps<{
    item: PrivacyItem;
    audiences: PrivacyOption<Audience>[];
    disabled?: boolean;
}>();
const emit = defineEmits<{ change: [audience: Audience | 'default'] }>();

const options = computed(() => [
    ...props.audiences.map((a) => ({
        value: a.value,
        label: a.value === props.item.default ? `${a.label} (default)` : a.label,
    })),
]);

const selected = computed(() => props.item.audience);

function onChange(value: string | number) {
    const v = String(value) as Audience;
    emit('change', v === props.item.default ? 'default' : v);
}
</script>

<template>
    <div class="flex flex-col gap-2 py-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="min-w-0">
            <div class="flex items-center gap-2 text-xs font-medium text-white">
                {{ item.label }}
                <span
                    v-if="!item.is_default"
                    class="text-lava font-mono text-[9px] tracking-wider uppercase"
                >
                    Custom
                </span>
            </div>
            <p class="text-dim text-[11px]">{{ item.description }}</p>
        </div>
        <div
            class="w-full shrink-0 sm:w-44"
            :class="disabled ? 'pointer-events-none opacity-50' : ''"
        >
            <UiSelectMenu
                :model-value="selected"
                :options="options"
                @update:model-value="onChange"
            />
        </div>
    </div>
</template>
