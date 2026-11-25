<script setup lang="ts">
import type { RoleSummary } from '~/types/permissions';

// Multi-select of a role's parents (checkbox list of the roles that would not make a cycle).
const props = defineProps<{
    candidates: RoleSummary[];
}>();

const model = defineModel<number[]>({ required: true });

const toggle = (id: number) => {
    model.value = model.value.includes(id)
        ? model.value.filter((p) => p !== id)
        : [...model.value, id];
};

const empty = computed(() => props.candidates.length === 0);
</script>

<template>
    <div class="border-border bg-void/60 max-h-40 overflow-y-auto rounded-lg border p-1">
        <div v-if="empty" class="text-dim px-2 py-1.5 text-[11px]">No other role</div>
        <label
            v-for="role in candidates"
            :key="role.id"
            class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 transition-colors
                hover:bg-white/5"
        >
            <input
                type="checkbox"
                :checked="model.includes(role.id)"
                class="accent-lava h-3 w-3 rounded"
                @change="toggle(role.id)"
            />
            <span class="text-[11px] text-white">{{ role.name }}</span>
            <span v-if="role.is_superuser" class="text-lava text-[9px] uppercase">superuser</span>
            <span v-if="role.description" class="text-dim truncate text-[10px]">
                {{ role.description }}
            </span>
        </label>
    </div>
</template>
