<script setup lang="ts">
import type { NavBarId } from '~/types/navLayout';

// The three zones of one bar in the editor, each a drop area.
const props = defineProps<{ bar: NavBarId }>();
const editor = useNavLayoutEditorContext();

const zones = computed(() =>
    NAV_ZONES.map((zone) => ({
        zone,
        label: NAV_ZONE_LABELS[zone],
        loc: { kind: 'zone' as const, bar: props.bar, zone },
        entries: editor.draft.value.bars[props.bar][zone],
    })),
);
</script>

<template>
    <section>
        <h4 class="text-dim mb-2 text-[11px] font-medium tracking-wider uppercase">
            {{ NAV_BAR_LABELS[bar] }}
        </h4>
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
            <div v-for="z in zones" :key="z.zone" class="min-w-0">
                <div class="mb-1 text-[10px] text-white/40">{{ z.label }}</div>
                <SettingsNavigationEntryList
                    :loc="z.loc"
                    :entries="z.entries"
                    :label="`${NAV_BAR_LABELS[bar]}, ${z.label}`"
                />
            </div>
        </div>
    </section>
</template>
