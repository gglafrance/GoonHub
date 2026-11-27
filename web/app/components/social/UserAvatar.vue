<script setup lang="ts">
import type { AvatarSize } from '~/utils/profile';

// Avatar of a user: the uploaded image when there is one (versioned URL), else initials.
const props = withDefaults(
    defineProps<{
        username: string;
        displayName?: string;
        userId?: number;
        avatarVersion?: number;
        size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl';
    }>(),
    { size: 'sm', displayName: '', userId: undefined, avatarVersion: 0 },
);

const BOX: Record<string, string> = {
    xs: 'h-6 w-6 text-[9px]',
    sm: 'h-8 w-8 text-xs',
    md: 'h-11 w-11 text-sm',
    lg: 'h-14 w-14 text-xl',
    xl: 'h-24 w-24 text-3xl sm:h-28 sm:w-28',
};
const IMAGE: Record<string, AvatarSize> = { xs: 'sm', sm: 'sm', md: 'md', lg: 'md', xl: 'lg' };

const failed = ref(false);
const src = computed(() =>
    failed.value ? null : avatarUrl(props.userId, props.avatarVersion, IMAGE[props.size]),
);
watch(
    () => [props.userId, props.avatarVersion],
    () => (failed.value = false),
);
const initials = computed(() => avatarInitials(props.displayName || props.username));
</script>

<template>
    <div
        class="border-border bg-panel flex shrink-0 items-center justify-center overflow-hidden
            rounded-full border"
        :class="BOX[size]"
    >
        <img
            v-if="src"
            :src="src"
            :alt="username"
            loading="lazy"
            class="h-full w-full object-cover"
            @error="failed = true"
        />
        <span v-else class="font-semibold text-white">{{ initials }}</span>
    </div>
</template>
