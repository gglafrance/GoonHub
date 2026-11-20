<script setup lang="ts">
import type { Component } from 'vue';
import type { HomepageSection, HomepageSectionData } from '~/types/homepage';
import { sectionTypeComponentName } from '~/types/homepage';

// Renders a widget section with components/homepage/widgets/<PascalType>.vue.
// Widget component contract: props { section, data }, emits refresh (reload the section data).
const props = defineProps<{
    section: HomepageSection;
    data: HomepageSectionData;
}>();

const emit = defineEmits<{
    refresh: [];
}>();

const widgetModules = import.meta.glob('./widgets/*.vue');

const widget = computed<Component | null>(() => {
    const loader = widgetModules[`./widgets/${sectionTypeComponentName(props.section.type)}.vue`];
    return loader ? defineAsyncComponent(loader as () => Promise<Component>) : null;
});
</script>

<template>
    <component
        :is="widget"
        v-if="widget"
        :section="section"
        :data="data"
        @refresh="emit('refresh')"
    />
    <div
        v-else
        class="border-border text-dim flex h-24 items-center justify-center rounded-lg border
            border-dashed text-xs"
    >
        Widget "{{ section.type }}" is not available yet
    </div>
</template>
