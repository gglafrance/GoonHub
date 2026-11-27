<script setup lang="ts">
const serveOGMetadata = defineModel<boolean>('serveOgMetadata', { required: true });
const transcodingEnabled = defineModel<boolean>('transcodingEnabled', { required: true });
</script>

<template>
    <div class="space-y-6">
        <div class="glass-panel p-5">
            <h3 class="mb-2 text-sm font-semibold text-white">App Settings</h3>
            <p class="text-dim mb-4 text-xs">Global application settings.</p>

            <div class="flex items-center justify-between">
                <div>
                    <label class="text-sm font-medium text-white"> Link Preview Metadata </label>
                    <p class="text-dim mt-0.5 text-xs">
                        Serve OpenGraph meta tags to social media crawlers for rich link previews
                        (Discord, Twitter, etc.)
                    </p>
                </div>
                <UiToggle v-model="serveOGMetadata" />
            </div>

            <div class="border-border mt-4 flex items-center justify-between border-t pt-4">
                <div>
                    <label class="text-sm font-medium text-white"> On-the-fly Transcoding </label>
                    <p class="text-dim mt-0.5 text-xs">
                        Convert the formats browsers can't play (AVI, MPEG-2, MKV, HEVC, WMV...)
                        while they are watched. Uses server CPU; when off, these scenes can only be
                        downloaded
                    </p>
                </div>
                <UiToggle v-model="transcodingEnabled" />
            </div>
        </div>

        <SettingsAppTranscodeCache />
    </div>
</template>
