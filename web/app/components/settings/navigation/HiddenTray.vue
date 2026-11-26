<script setup lang="ts">
// Items the user hid: drop an item here to hide it (not the required ones).
const editor = useNavLayoutEditorContext();
</script>

<template>
    <section>
        <h4 class="text-dim mb-2 text-[11px] font-medium tracking-wider uppercase">Hidden</h4>
        <SettingsNavigationEntryList
            :loc="{ kind: 'hidden' }"
            :entries="editor.draft.value.hidden"
            label="Hidden items"
            empty-label="Drop an item here to hide it. The logo, Settings and Logout always stay."
        />
    </section>
</template>
