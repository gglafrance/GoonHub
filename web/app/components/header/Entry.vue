<script setup lang="ts">
import type { NavItemDef } from '~/utils/navRegistry';

// One item of a header bar, rendered by its own component (the same in both bars). Items and
// permissions: utils/navRegistry.ts; the layout decides where they go.
defineProps<{ item: NavItemDef }>();
</script>

<template>
    <HeaderLogo v-if="item.id === 'logo'" />
    <HeaderCommandPaletteButton v-else-if="item.id === 'command_palette'" />
    <HeaderProfileChip v-else-if="item.id === 'profile'" />
    <HeaderShortcutsButton v-else-if="item.id === 'shortcuts'" />
    <HeaderSafeModeButton v-else-if="item.id === 'safe_mode'" />
    <HeaderHoverAudioControl v-else-if="item.id === 'preview_sound'" />
    <HeaderJobStatus v-else-if="item.id === 'job_status'" />
    <HeaderFriendsPresence v-else-if="item.id === 'presence'" />
    <HeaderMessages v-else-if="item.id === 'messages'" />
    <HeaderFriendsLink v-else-if="item.id === 'friends'" />
    <HeaderSettingsLink v-else-if="item.id === 'settings'" />
    <HeaderLogoutButton v-else-if="item.id === 'logout'" />
    <HeaderNavLink v-else-if="item.to" :item="item" />
</template>
