<script setup lang="ts">
import type { Profile, ProfileRatingsPayload } from '~/types/profile';

// Ratings tab of a profile page: the ratings chart and the videos the user rated, filtered by
// one bar of the chart (?rating=, a click on a bar toggles it) (privacy item `ratings`).
const props = defineProps<{ profile: Profile; owner: boolean }>();

definePageMeta({
    middleware: ['auth'],
});

useHead({ title: () => `Ratings · ${displayNameOf(props.profile.user)}` });

const api = useApiProfiles();

const SORTS = [
    { value: 'rating_desc', label: 'Highest rated' },
    { value: 'rating_asc', label: 'Lowest rated' },
    { value: 'rated_desc', label: 'Recently rated' },
    { value: 'rated_asc', label: 'First rated' },
];

const rating = useUrlQueryRef<number | null>(
    'rating',
    null,
    (raw) => parseRatingQuery(raw) ?? undefined,
    (v) => (v == null ? '' : String(v)),
);

const histogram = ref<ProfileRatingsPayload | null>(null);

async function loadHistogram() {
    if (!props.profile.lists?.ratings) return;
    try {
        histogram.value = await api.getRatingHistogram(props.profile.user.username);
    } catch {
        histogram.value = null;
    }
}

function toggle(value: number) {
    rating.value = rating.value === value ? null : value;
}

watch(() => props.profile.user.username, loadHistogram);
onMounted(loadHistogram);
</script>

<template>
    <section class="glass-panel space-y-4 p-4">
        <template v-if="props.profile.lists?.ratings">
            <div v-if="histogram && histogram.total > 0" class="max-w-md">
                <div class="border-border mb-3 flex items-baseline border-b pb-1.5">
                    <h2 class="text-dim text-[11px] font-semibold tracking-wider uppercase">
                        Ratings
                    </h2>
                    <span class="text-dim ml-auto font-mono text-[11px]">{{
                        histogram.total
                    }}</span>
                </div>
                <ProfileRatingHistogram
                    :buckets="histogram.buckets"
                    :total="histogram.total"
                    :average="histogram.average"
                    :selected="rating"
                    @select="toggle"
                />
            </div>
            <ProfileSceneList
                :username="props.profile.user.username"
                kind="ratings"
                :sort-options="SORTS"
                figure="rating"
                :rating="rating"
                :empty-text="
                    rating != null
                        ? `No video rated ${ratingLabel(rating)}.`
                        : props.owner
                          ? 'You have not rated any video yet.'
                          : 'No rated video yet.'
                "
            >
                <template #toolbar>
                    <button
                        v-if="rating != null"
                        type="button"
                        class="border-lava/40 bg-lava/10 text-lava hover:bg-lava/20 flex
                            items-center gap-1 rounded-full border px-2.5 py-1 text-[11px]
                            transition-colors"
                        :aria-label="`Remove the ${ratingLabel(rating)} filter`"
                        @click="rating = null"
                    >
                        {{ ratingStars(rating) }}
                        <span class="font-mono">{{ rating }}</span>
                        <Icon name="heroicons:x-mark" size="12" />
                    </button>
                </template>
            </ProfileSceneList>
        </template>
        <ProfileListLocked v-else />
    </section>
</template>
