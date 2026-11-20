<script setup lang="ts">
// Timezone field shared by the stats widget config forms. An empty value is filled with the
// browser timezone so that hours and days match the user's clock.
const tz = defineModel<string>({ default: '' });

const browserTz = (() => {
    try {
        return Intl.DateTimeFormat().resolvedOptions().timeZone || '';
    } catch {
        return '';
    }
})();

onMounted(() => {
    if (!tz.value && browserTz) tz.value = browserTz;
});
</script>

<template>
    <div>
        <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
            Timezone
        </label>
        <div class="flex gap-2">
            <input
                v-model.trim="tz"
                type="text"
                maxlength="64"
                placeholder="Europe/Paris"
                class="border-border bg-void/80 focus:border-lava/40 flex-1 rounded-lg border px-3
                    py-2 font-mono text-xs text-white focus:outline-none"
            />
            <button
                v-if="browserTz && tz !== browserTz"
                class="border-border text-dim rounded-lg border px-2.5 text-[11px] transition-colors
                    hover:text-white"
                @click="tz = browserTz"
            >
                Use browser ({{ browserTz }})
            </button>
        </div>
        <p class="text-dim mt-1 text-[10px]">
            IANA name used for hours and days (empty: server timezone).
        </p>
    </div>
</template>
