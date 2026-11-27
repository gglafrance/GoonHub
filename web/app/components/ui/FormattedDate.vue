<script setup lang="ts">
import type { DateInput } from '~/utils/dateFormat';

// Absolute date following the user's date format setting. Use <NuxtTime relative /> for relative times.
const props = defineProps<{
    datetime: DateInput;
    /** Also show the time */
    time?: boolean;
    /** Calendar date (release date): shown in UTC so the day never shifts */
    utc?: boolean;
}>();

const { formatDate, formatDateTime } = useDateFormatter();

const date = computed(() => toDate(props.datetime));
const text = computed(() => formatDate(props.datetime, { time: props.time, utc: props.utc }));
const title = computed(() => (props.utc ? text.value : formatDateTime(props.datetime)));
</script>

<template>
    <time v-if="date" :datetime="date.toISOString()" :title="title">{{ text }}</time>
</template>
