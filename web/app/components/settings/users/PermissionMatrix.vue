<script setup lang="ts">
import type {
    CatalogPermission,
    EffectivePermission,
    PermissionChoice,
    PermissionSetting,
} from '~/types/permissions';

// Permissions grouped by catalog group: a tri-state setting per permission and its effective
// result with the source. Used by the role editor and the per-user overrides.
const props = defineProps<{
    catalog: CatalogPermission[];
    settings: PermissionSetting[];
    effective: EffectivePermission[];
    readonly?: boolean;
    // Superuser role: everything granted, nothing can be set
    superuser?: boolean;
    inheritLabel?: string;
}>();

const emit = defineEmits<{
    change: [permissionId: number, choice: PermissionChoice];
}>();

const effectiveMap = computed(() => effectiveByName(props.effective));

const effectiveOf = (name: string): EffectivePermission | null => {
    if (props.superuser) return { name, granted: true, source: 'superuser' };
    return effectiveMap.value.get(name) ?? null;
};

const groups = computed(() =>
    groupPermissions(props.catalog).map((g) => ({
        group: g.group,
        rows: g.permissions.map((perm) => ({ perm, effective: effectiveOf(perm.name) })),
    })),
);
</script>

<template>
    <div class="space-y-4">
        <div v-for="group in groups" :key="group.group">
            <div class="text-dim mb-1.5 text-[10px] font-semibold tracking-wider uppercase">
                {{ group.group }}
            </div>
            <div class="border-border/50 divide-border/50 divide-y rounded-lg border">
                <div
                    v-for="{ perm, effective: eff } in group.rows"
                    :key="perm.id"
                    class="flex flex-wrap items-center gap-x-3 gap-y-1.5 px-3 py-2"
                >
                    <div class="min-w-0 flex-1">
                        <div class="font-mono text-[11px] text-white">{{ perm.name }}</div>
                        <div class="text-dim text-[10px]">{{ perm.description }}</div>
                    </div>
                    <SettingsUsersTriState
                        :model-value="superuser ? 'inherit' : choiceOf(settings, perm.id)"
                        :disabled="readonly || superuser"
                        :inherit-label="inheritLabel"
                        @update:model-value="emit('change', perm.id, $event)"
                    />
                    <div class="flex w-36 shrink-0 items-center gap-1.5 text-[10px]">
                        <template v-if="eff">
                            <Icon
                                :name="
                                    eff.granted ? 'heroicons:check-circle' : 'heroicons:no-symbol'
                                "
                                class="h-3.5 w-3.5 shrink-0"
                                :class="eff.granted ? 'text-emerald' : 'text-dim'"
                            />
                            <span :class="eff.granted ? 'text-white' : 'text-dim'">
                                {{ eff.granted ? 'Granted' : 'Refused' }}
                            </span>
                            <span class="text-dim truncate" :title="eff.source">
                                ({{ describeSource(eff.source) }})
                            </span>
                        </template>
                        <span v-else class="text-dim">-</span>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>
