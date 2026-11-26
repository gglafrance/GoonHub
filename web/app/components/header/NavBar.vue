<script setup lang="ts">
import type { PermissionName } from '~/types/permissions';

// Secondary nav row of the header: the user pages, then the "Manage" menu (curation and
// management pages) at its end. Items and their permissions: utils/navItems.ts.
const perms = useMyPermissions();
const jobStatusStore = useJobStatusStore();
const togetherLobbies = useTogetherLobbiesStore();

const has = (p: PermissionName) => perms.has(p);
const mainItems = computed(() => visibleNavItems(NAV_ITEMS, 'main', has));
const manageItems = computed(() => visibleNavItems(NAV_ITEMS, 'manage', has));

const canTogether = computed(() => perms.has('goon2gether:use'));
const publicLobbies = computed(() =>
    canTogether.value ? badgeCount(togetherLobbies.publicCount) : '',
);
const publicLobbiesTitle = computed(() => {
    const n = togetherLobbies.publicCount;
    return `${n} public lobb${n === 1 ? 'y' : 'ies'}`;
});

watch(
    canTogether,
    (can) => {
        if (can) togetherLobbies.start();
        else togetherLobbies.stop();
    },
    { immediate: true },
);
onBeforeUnmount(() => togetherLobbies.stop());

// Events may have been missed while the stream was down
watch(
    () => jobStatusStore.lastReconnectedAt,
    () => {
        if (canTogether.value) togetherLobbies.refresh();
    },
);
</script>

<template>
    <nav class="border-border bg-void/60 border-b backdrop-blur-sm">
        <div class="mx-auto max-w-415 px-4 sm:px-5">
            <div class="flex h-10 items-center justify-between sm:h-9 sm:justify-start sm:gap-1">
                <HeaderNavLink
                    v-for="item in mainItems"
                    :key="item.to"
                    :item="item"
                    :badge="item.to === '/together' ? publicLobbies : undefined"
                    :badge-title="
                        item.to === '/together' && publicLobbies ? publicLobbiesTitle : undefined
                    "
                />
                <HeaderManageMenu v-if="manageItems.length > 0" :items="manageItems" />
            </div>
        </div>
    </nav>
</template>
