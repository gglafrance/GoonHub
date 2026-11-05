<script setup lang="ts">
const props = withDefaults(
    defineProps<{
        visible: boolean;
        title: string;
        confirmLabel?: string;
        icon?: string;
    }>(),
    {
        confirmLabel: 'Confirm',
        icon: 'heroicons:exclamation-triangle',
    },
);

const emit = defineEmits<{
    close: [];
    confirm: [];
}>();

const handleEscape = (e: KeyboardEvent) => {
    if (props.visible && e.key === 'Escape') emit('close');
};

onMounted(() => window.addEventListener('keydown', handleEscape));
onUnmounted(() => window.removeEventListener('keydown', handleEscape));
</script>

<template>
    <Teleport to="body">
        <Transition
            enter-active-class="transition duration-200 ease-out"
            enter-from-class="opacity-0"
            enter-to-class="opacity-100"
            leave-active-class="transition duration-150 ease-in"
            leave-from-class="opacity-100"
            leave-to-class="opacity-0"
        >
            <div
                v-if="visible"
                class="fixed inset-0 z-50 flex items-center justify-center bg-black/60
                    backdrop-blur-sm"
                @click.self="emit('close')"
            >
                <div
                    class="border-border bg-panel mx-4 w-full max-w-sm rounded-xl border p-5
                        shadow-2xl"
                >
                    <div class="mb-4 flex items-start gap-3">
                        <div class="bg-lava/10 flex items-center justify-center rounded-full p-2">
                            <Icon :name="icon" size="20" class="text-lava" />
                        </div>
                        <div>
                            <h3 class="text-sm font-medium text-white">{{ title }}</h3>
                            <div class="text-dim mt-1 space-y-2 text-xs">
                                <slot />
                            </div>
                        </div>
                    </div>
                    <div class="flex justify-end gap-2">
                        <button
                            class="border-border hover:bg-surface-hover rounded-lg border px-3
                                py-1.5 text-xs text-white transition-colors"
                            @click="emit('close')"
                        >
                            Cancel
                        </button>
                        <button
                            class="bg-lava hover:bg-lava-glow rounded-lg px-3 py-1.5 text-xs
                                font-semibold text-white transition-colors"
                            @click="emit('confirm')"
                        >
                            {{ confirmLabel }}
                        </button>
                    </div>
                </div>
            </div>
        </Transition>
    </Teleport>
</template>
