<script setup lang="ts">
// Config of the folder_latest widget: storage path and/or folder.
const config = defineModel<Record<string, unknown>>({ required: true });

const folder = computed({
    get: () => String(config.value.folder ?? ''),
    set: (v: string) => {
        const next = { ...config.value };
        if (v.trim()) next.folder = v.trim();
        else delete next.folder;
        config.value = next;
    },
});

const valid = computed(() => !!config.value.storage_path_id || !!folder.value);
</script>

<template>
    <div class="space-y-3">
        <SettingsHomepageWidgetConfigScenesStoragePathSelect v-model="config" />
        <SettingsHomepageWidgetConfigScenesField
            label="Folder"
            hint="An absolute path (/media/stash/coomer-dl/...) or a folder name found anywhere in the file path."
        >
            <input
                v-model.lazy="folder"
                type="text"
                maxlength="500"
                placeholder="e.g. onlyfans or /media/stash/data2"
                class="border-border bg-void/80 focus:border-lava/40 w-full rounded-lg border px-3
                    py-2 font-mono text-xs text-white focus:outline-none"
            />
        </SettingsHomepageWidgetConfigScenesField>
        <p v-if="!valid" class="text-lava text-[11px]">Choose a storage path or a folder.</p>
    </div>
</template>
