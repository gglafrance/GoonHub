<script setup lang="ts">
import type { ProfileBlock, ProfileRatingsPayload } from '~/types/profile';

// ratings: the chart of the owner's ratings (profile block). A bar opens the videos rated
// with that value.
const props = defineProps<{ block: ProfileBlock<ProfileRatingsPayload>; owner: boolean }>();

const route = useRoute();
const username = computed(() => String(route.params.username ?? ''));

function openRating(rating: number) {
    navigateTo(profileTabPath(username.value, 'ratings', { rating: String(rating) }));
}
</script>

<template>
    <ProfileRatingHistogram
        v-if="props.block.widget"
        :buckets="props.block.widget.buckets"
        :total="props.block.widget.total"
        :average="props.block.widget.average"
        class="max-w-md"
        @select="openRating"
    />
</template>
