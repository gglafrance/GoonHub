<script setup lang="ts">
defineProps<{
    title: string;
    icon: string;
    collapsed: boolean;
    badge?: string | number;
}>();

const emit = defineEmits<{
    toggle: [];
}>();
</script>

<template>
    <div class="border-border rounded-md border">
        <button
            class="flex w-full items-center justify-between px-2.5 py-2"
            @click="emit('toggle')"
        >
            <div class="flex items-center gap-1.5">
                <Icon :name="icon" size="12" class="text-dim" />
                <span class="text-dim text-[11px] font-medium uppercase">{{ title }}</span>
            </div>
            <div class="flex items-center gap-1.5">
                <span
                    v-if="badge"
                    class="bg-lava/20 text-lava max-w-20 truncate rounded-full px-1.5 text-[10px]
                        font-medium"
                >
                    {{ badge }}
                </span>
                <Icon
                    :name="collapsed ? 'heroicons:chevron-down' : 'heroicons:chevron-up'"
                    size="12"
                    class="text-dim"
                />
            </div>
        </button>
        <div v-show="!collapsed" class="border-border border-t px-2.5 py-2.5">
            <slot />
        </div>
    </div>
</template>
