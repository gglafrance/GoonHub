<script setup lang="ts">
const searchStore = useSearchStore();

const collapsed = ref(true);
const showHelp = ref(false);

const badge = computed(() => {
    if (searchStore.matchType === 'strict') return 'Strict';
    if (searchStore.matchType === 'frequency') return 'Frequency';
    return undefined;
});

const matchOptions = [
    {
        value: 'broad' as const,
        label: 'Broad',
        description: 'Flexible matching. May remove terms from the end if needed to find results.',
        example: '"action comedy" may match videos with just "action"',
    },
    {
        value: 'strict' as const,
        label: 'Strict',
        description: 'All query terms must be present in the results.',
        example: '"action comedy" only matches videos with both terms',
    },
    {
        value: 'frequency' as const,
        label: 'Frequency',
        description: 'Prioritizes rare/uncommon terms in your search.',
        example: 'Unique names rank higher than common words',
    },
];
</script>

<template>
    <SearchFiltersFilterSection
        title="Match Type"
        icon="heroicons:adjustments-horizontal"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <div class="space-y-2">
            <label
                v-for="option in matchOptions"
                :key="option.value"
                class="flex cursor-pointer items-start gap-2"
            >
                <input
                    v-model="searchStore.matchType"
                    type="radio"
                    :value="option.value"
                    class="accent-lava mt-0.5 h-3.5 w-3.5"
                />
                <div class="flex-1">
                    <span class="text-xs font-medium text-white">{{ option.label }}</span>
                    <p class="text-dim text-[10px] leading-snug">{{ option.description }}</p>
                </div>
            </label>
        </div>

        <!-- Help panel toggle -->
        <button
            @click="showHelp = !showHelp"
            class="text-dim hover:text-lava mt-3 flex items-center gap-1 text-[10px]
                transition-colors"
        >
            <Icon
                :name="showHelp ? 'heroicons:chevron-up' : 'heroicons:information-circle'"
                size="12"
            />
            {{ showHelp ? 'Hide search tips' : 'Search tips' }}
        </button>

        <!-- Help panel -->
        <div v-if="showHelp" class="mt-2 rounded-md bg-white/5 p-2.5">
            <h4 class="mb-1.5 text-[10px] font-semibold tracking-wide text-white uppercase">
                Quote Syntax
            </h4>
            <div class="space-y-2 text-[10px]">
                <div>
                    <code class="text-lava bg-lava/10 rounded px-1">"exact phrase"</code>
                    <p class="text-dim mt-0.5 leading-snug">
                        Find videos containing this exact phrase
                    </p>
                </div>
                <div>
                    <code class="text-lava bg-lava/10 rounded px-1">"term1" "term2"</code>
                    <p class="text-dim mt-0.5 leading-snug">
                        AND search - both terms must appear (works best with Strict mode)
                    </p>
                </div>
            </div>
        </div>
    </SearchFiltersFilterSection>
</template>
