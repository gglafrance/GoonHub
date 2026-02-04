<script setup lang="ts">
const props = withDefaults(
    defineProps<{
        visible: boolean;
        title?: string;
        description?: string;
    }>(),
    {
        title: 'Save Preset',
        description: 'Give your preset a memorable name.',
    },
);

const emit = defineEmits<{
    close: [];
    save: [name: string];
}>();

const name = ref('');
const inputRef = ref<HTMLInputElement | null>(null);

watch(
    () => props.visible,
    (val) => {
        if (val) {
            name.value = '';
            nextTick(() => inputRef.value?.focus());
        }
    },
);

function handleSave() {
    if (name.value.trim()) {
        emit('save', name.value.trim());
    }
}

function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && name.value.trim()) {
        handleSave();
    } else if (e.key === 'Escape') {
        emit('close');
    }
}
</script>

<template>
    <Teleport to="body">
        <Transition
            enter-active-class="transition-all duration-200 ease-out"
            enter-from-class="opacity-0"
            enter-to-class="opacity-100"
            leave-active-class="transition-all duration-150 ease-in"
            leave-from-class="opacity-100"
            leave-to-class="opacity-0"
        >
            <div
                v-if="visible"
                class="fixed inset-0 z-60 flex items-center justify-center bg-black/80
                    backdrop-blur-sm"
                @click.self="$emit('close')"
                @keydown="handleKeydown"
            >
                <Transition
                    enter-active-class="transition-all duration-200 ease-out"
                    enter-from-class="opacity-0 scale-95 translate-y-4"
                    enter-to-class="opacity-100 scale-100 translate-y-0"
                    leave-active-class="transition-all duration-150 ease-in"
                    leave-from-class="opacity-100 scale-100 translate-y-0"
                    leave-to-class="opacity-0 scale-95 translate-y-4"
                >
                    <div
                        v-if="visible"
                        class="border-border bg-panel relative w-full max-w-sm overflow-hidden
                            rounded-2xl border shadow-2xl shadow-black/50"
                    >
                        <!-- Decorative header gradient -->
                        <div
                            class="from-lava/5 pointer-events-none absolute inset-x-0 top-0 h-24
                                bg-gradient-to-b to-transparent"
                        ></div>

                        <!-- Content -->
                        <div class="relative p-6">
                            <!-- Icon -->
                            <div
                                class="from-lava/20 to-lava/5 ring-lava/20 mx-auto mb-4 flex h-12
                                    w-12 items-center justify-center rounded-xl bg-gradient-to-br
                                    ring-1"
                            >
                                <Icon name="heroicons:bookmark" size="24" class="text-lava" />
                            </div>

                            <!-- Title -->
                            <h3 class="mb-1 text-center text-base font-semibold text-white">
                                {{ title }}
                            </h3>
                            <p class="text-dim mb-5 text-center text-xs">
                                {{ description }}
                            </p>

                            <!-- Input -->
                            <div class="mb-5">
                                <label
                                    class="text-dim mb-2 flex items-center gap-1.5 text-[11px]
                                        font-medium tracking-wider uppercase"
                                >
                                    <Icon name="heroicons:tag" size="11" />
                                    Preset Name
                                </label>
                                <input
                                    ref="inputRef"
                                    v-model="name"
                                    type="text"
                                    placeholder="My Custom Preset"
                                    maxlength="100"
                                    class="border-border bg-void/80 placeholder:text-dim/40
                                        focus:border-lava/40 focus:ring-lava/20 w-full rounded-lg
                                        border px-4 py-3 text-sm text-white transition-all
                                        focus:ring-1 focus:outline-none"
                                />
                                <p class="text-dim/50 mt-1.5 text-right text-[10px] tabular-nums">
                                    {{ name.length }}/100
                                </p>
                            </div>

                            <!-- Actions -->
                            <div class="flex gap-3">
                                <button
                                    class="border-border hover:border-border-hover flex-1 rounded-lg
                                        border px-4 py-2.5 text-xs font-medium text-white
                                        transition-all hover:bg-white/5"
                                    @click="$emit('close')"
                                >
                                    Cancel
                                </button>
                                <button
                                    :disabled="!name.trim()"
                                    class="bg-lava hover:bg-lava-glow hover:shadow-lava/20
                                        disabled:hover:bg-lava flex flex-1 items-center
                                        justify-center gap-2 rounded-lg px-4 py-2.5 text-xs
                                        font-semibold text-white transition-all hover:shadow-lg
                                        disabled:cursor-not-allowed disabled:opacity-50
                                        disabled:hover:shadow-none"
                                    @click="handleSave"
                                >
                                    <Icon name="heroicons:check" size="14" />
                                    Save Preset
                                </button>
                            </div>
                        </div>
                    </div>
                </Transition>
            </div>
        </Transition>
    </Teleport>
</template>
