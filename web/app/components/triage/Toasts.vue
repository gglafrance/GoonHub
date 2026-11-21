<script setup lang="ts">
import type { TriageToast } from '~/composables/useTriageToasts';

defineProps<{
    toasts: readonly TriageToast[];
}>();

const emit = defineEmits<{
    action: [toast: TriageToast];
    dismiss: [id: number];
}>();
</script>

<template>
    <Teleport to="body">
        <div
            class="pointer-events-none fixed bottom-4 left-1/2 z-90 flex w-full max-w-sm
                -translate-x-1/2 flex-col items-center gap-2 px-4"
            aria-live="polite"
        >
            <TransitionGroup
                enter-active-class="transition duration-150 ease-out"
                enter-from-class="translate-y-2 opacity-0"
                leave-active-class="transition duration-150 ease-in"
                leave-to-class="opacity-0"
            >
                <div
                    v-for="toast in toasts"
                    :key="toast.id"
                    class="bg-panel/90 pointer-events-auto flex w-full items-center gap-2 rounded-lg
                        border px-3 py-2 text-xs shadow-2xl backdrop-blur-md"
                    :class="
                        toast.tone === 'error'
                            ? 'border-lava/40 text-red-200'
                            : toast.tone === 'info'
                              ? 'border-border text-white'
                              : 'border-emerald/30 text-white'
                    "
                >
                    <Icon
                        :name="
                            toast.icon ||
                            (toast.tone === 'error'
                                ? 'heroicons:exclamation-triangle'
                                : 'heroicons:check')
                        "
                        size="14"
                        class="shrink-0"
                        :class="
                            toast.tone === 'error'
                                ? 'text-lava'
                                : toast.tone === 'info'
                                  ? 'text-muted'
                                  : 'text-emerald'
                        "
                    />
                    <span class="min-w-0 flex-1 truncate">{{ toast.message }}</span>
                    <button
                        v-if="toast.actionLabel"
                        class="text-lava hover:bg-lava/10 shrink-0 rounded px-1.5 py-0.5
                            font-semibold transition-colors"
                        @click="emit('action', toast)"
                    >
                        {{ toast.actionLabel }}
                    </button>
                    <button
                        class="text-dim shrink-0 transition-colors hover:text-white"
                        title="Dismiss"
                        @click="emit('dismiss', toast.id)"
                    >
                        <Icon name="heroicons:x-mark" size="12" />
                    </button>
                </div>
            </TransitionGroup>
        </div>
    </Teleport>
</template>
