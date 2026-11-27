<script setup lang="ts">
import type { RoleSummary } from '~/types/permissions';

defineProps<{
    role: RoleSummary;
    parents: string[];
    editable: boolean;
}>();

defineEmits<{
    edit: [];
    permissions: [];
    delete: [];
}>();
</script>

<template>
    <div
        class="border-border/50 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border px-3
            py-2.5"
    >
        <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-1.5">
                <span
                    class="inline-block rounded-full border px-2 py-0.5 text-[10px] font-medium"
                    :class="roleBadgeClass(role)"
                >
                    {{ role.name }}
                </span>
                <span
                    v-if="role.is_superuser"
                    class="border-lava/30 text-lava rounded border px-1.5 py-px text-[9px]
                        font-semibold tracking-wider uppercase"
                    title="Has every permission, nothing can be denied"
                >
                    Superuser
                </span>
                <span
                    v-if="role.is_system"
                    class="text-dim rounded border border-white/10 px-1.5 py-px text-[9px]
                        font-semibold tracking-wider uppercase"
                    title="Built-in role, cannot be deleted"
                >
                    System
                </span>
                <span class="text-dim text-[11px]">
                    {{ role.user_count }} {{ role.user_count === 1 ? 'user' : 'users' }}
                </span>
            </div>
            <div v-if="role.description" class="text-dim mt-1 text-[11px]">
                {{ role.description }}
            </div>
            <div
                v-if="parents.length > 0"
                class="text-dim mt-1 flex items-center gap-1 text-[10px]"
            >
                <Icon name="heroicons:arrow-turn-down-right" class="h-3 w-3" />
                Inherits from
                <span class="text-white/70">{{ parents.join(', ') }}</span>
            </div>
        </div>
        <div class="flex shrink-0 gap-3">
            <button
                class="text-dim text-[11px] transition-colors hover:text-white"
                @click="$emit('permissions')"
            >
                Permissions
            </button>
            <template v-if="editable">
                <button
                    class="text-dim text-[11px] transition-colors hover:text-white"
                    @click="$emit('edit')"
                >
                    Edit
                </button>
                <button
                    :disabled="role.is_system"
                    :title="role.is_system ? 'Built-in role, cannot be deleted' : undefined"
                    class="text-lava/70 hover:text-lava disabled:hover:text-lava/70 text-[11px]
                        transition-colors disabled:cursor-not-allowed disabled:opacity-40"
                    @click="$emit('delete')"
                >
                    Delete
                </button>
            </template>
        </div>
    </div>
</template>
