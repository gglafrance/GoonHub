<script setup lang="ts">
import type { AdminUser } from '~/types/admin';
import type { RoleSummary } from '~/types/permissions';

defineProps<{
    user: AdminUser;
    // The user's role when known (badge style)
    role: RoleSummary | null;
    shareSaving: boolean;
}>();

defineEmits<{
    shareMarkers: [share: boolean];
    editRole: [];
    permissions: [];
    resetPassword: [];
    delete: [];
}>();

const { formatDateTime } = useDateFormatter();

const formatDate = (dateStr: string | null): string =>
    dateStr ? formatDateTime(dateStr) : 'Never';
</script>

<template>
    <tr class="border-border/50 border-b last:border-0">
        <td class="py-2.5 pr-4 text-white">{{ user.username }}</td>
        <td class="py-2.5 pr-4">
            <span
                class="inline-block rounded-full border px-2 py-0.5 text-[10px] font-medium"
                :class="roleBadgeClass(role ?? user.role)"
            >
                {{ user.role }}
            </span>
        </td>
        <td class="py-2.5 pr-4">
            <UiToggle
                :model-value="user.share_markers"
                :disabled="shareSaving"
                :title="
                    user.share_markers
                        ? `Every user sees ${user.username}'s markers`
                        : `Only ${user.username} sees their markers`
                "
                @update:model-value="$emit('shareMarkers', $event)"
            />
        </td>
        <td class="text-dim py-2.5 pr-4">{{ formatDate(user.created_at) }}</td>
        <td class="text-dim py-2.5 pr-4">{{ formatDate(user.last_login_at) }}</td>
        <td class="py-2.5">
            <div class="flex gap-2">
                <button
                    class="text-dim text-[11px] transition-colors hover:text-white"
                    @click="$emit('editRole')"
                >
                    Role
                </button>
                <button
                    class="text-dim text-[11px] transition-colors hover:text-white"
                    title="Per-user permission overrides"
                    @click="$emit('permissions')"
                >
                    Permissions
                </button>
                <button
                    class="text-dim text-[11px] transition-colors hover:text-white"
                    @click="$emit('resetPassword')"
                >
                    Password
                </button>
                <button
                    class="text-lava/70 hover:text-lava text-[11px] transition-colors"
                    @click="$emit('delete')"
                >
                    Delete
                </button>
            </div>
        </td>
    </tr>
</template>
