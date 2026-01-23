<script setup lang="ts">
defineProps<{
    message: string;
}>();
</script>

<template>
    <div class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs">
        {{ message }}
    </div>
</template>
