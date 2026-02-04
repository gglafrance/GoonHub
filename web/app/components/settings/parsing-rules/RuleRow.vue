<script setup lang="ts">
import type { ParsingRule, ParsingRuleType } from '~/types/parsing-rules';
import { RULE_TYPE_INFO } from '~/types/parsing-rules';

const props = defineProps<{
    rule: ParsingRule;
    index: number;
    isDragging?: boolean;
}>();

const emit = defineEmits<{
    update: [rule: ParsingRule];
    delete: [];
    toggle: [];
}>();

const { validateRegex } = useParsingRulesEngine();

const ruleInfo = computed(() => RULE_TYPE_INFO[props.rule.type as ParsingRuleType]);
const regexError = ref<string | null>(null);
const isExpanded = ref(ruleInfo.value.hasConfig);

// Rule type icons
const ruleIcons: Record<ParsingRuleType, string> = {
    remove_brackets: 'heroicons:code-bracket',
    remove_numbers: 'heroicons:hashtag',
    remove_years: 'heroicons:calendar',
    remove_special_chars: 'heroicons:minus',
    remove_stopwords: 'heroicons:x-mark',
    remove_duplicates: 'heroicons:document-duplicate',
    regex_remove: 'heroicons:command-line',
    text_replace: 'heroicons:arrows-right-left',
    word_length_filter: 'heroicons:adjustments-horizontal',
    case_normalize: 'heroicons:language',
};

function updateConfig<K extends keyof ParsingRule['config']>(
    key: K,
    value: ParsingRule['config'][K],
) {
    const updated: ParsingRule = {
        ...props.rule,
        config: { ...props.rule.config, [key]: value },
    };

    // Validate regex if this is a regex_remove rule
    if (key === 'pattern' && props.rule.type === 'regex_remove') {
        const validation = validateRegex(value as string);
        regexError.value = validation.valid ? null : (validation.error ?? 'Invalid regex');
    }

    emit('update', updated);
}

function toggleEnabled() {
    emit('toggle');
}

// Get display text for rule
const ruleDisplayText = computed(() => {
    const { type, config } = props.rule;
    switch (type) {
        case 'remove_brackets':
            return config.keepContent ? 'keep content' : 'with content';
        case 'text_replace':
            if (config.find) {
                return `"${config.find}" → "${config.replace || ''}"`;
            }
            return 'Configure replacement';
        case 'regex_remove':
            return config.pattern ? `/${config.pattern}/` : 'Configure pattern';
        case 'word_length_filter':
            return config.minLength ? `min ${config.minLength} chars` : 'Configure filter';
        case 'case_normalize':
            return config.caseType || 'Configure case';
        default:
            return ruleInfo.value.description;
    }
});

// Get category color
const categoryColor = computed(() => {
    const type = props.rule.type;
    if (
        [
            'remove_brackets',
            'remove_numbers',
            'remove_years',
            'remove_special_chars',
            'remove_stopwords',
            'remove_duplicates',
        ].includes(type)
    ) {
        return 'text-rose-400';
    }
    if (['regex_remove', 'text_replace'].includes(type)) {
        return 'text-amber-400';
    }
    return 'text-sky-400';
});
</script>

<template>
    <div
        class="group relative transition-all duration-200"
        :class="[isDragging && 'z-10 scale-[1.02]', !rule.enabled && 'opacity-60']"
    >
        <div
            class="flex items-start gap-3 px-5 py-3.5 transition-all"
            :class="[isDragging ? 'bg-lava/5' : 'hover:bg-white/[0.02]']"
        >
            <!-- Drag handle and order number -->
            <div class="flex items-center gap-2 pt-0.5">
                <div
                    class="text-dim flex cursor-grab items-center justify-center transition-all
                        group-hover:text-white/50 active:cursor-grabbing"
                    :class="isDragging && 'text-lava'"
                >
                    <Icon name="heroicons:bars-2" size="14" />
                </div>
                <span class="text-dim w-4 text-center text-[10px] font-medium tabular-nums">
                    {{ index + 1 }}
                </span>
            </div>

            <!-- Toggle switch -->
            <button
                @click="toggleEnabled"
                class="relative mt-0.5 h-5 w-9 shrink-0 rounded-full transition-all duration-200"
                :class="
                    rule.enabled
                        ? 'bg-emerald-500/20 ring-1 ring-emerald-500/30'
                        : 'bg-white/5 ring-1 ring-white/10 hover:ring-white/20'
                "
            >
                <span
                    class="absolute top-0.5 left-0.5 h-4 w-4 rounded-full transition-all
                        duration-200"
                    :class="
                        rule.enabled
                            ? 'translate-x-4 bg-emerald-400 shadow-lg shadow-emerald-500/30'
                            : 'translate-x-0 bg-white/30'
                    "
                />
            </button>

            <!-- Rule info -->
            <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                    <!-- Rule icon -->
                    <div
                        class="flex h-6 w-6 shrink-0 items-center justify-center rounded-md
                            bg-white/5 ring-1 ring-white/5"
                        :class="rule.enabled ? categoryColor : 'text-dim'"
                    >
                        <Icon :name="ruleIcons[rule.type]" size="13" />
                    </div>

                    <!-- Rule name -->
                    <span
                        class="text-xs font-medium transition-colors"
                        :class="rule.enabled ? 'text-white' : 'text-white/50'"
                    >
                        {{ ruleInfo.label }}
                    </span>

                    <!-- Config summary badge -->
                    <span
                        v-if="ruleDisplayText && !ruleInfo.hasConfig"
                        class="text-dim truncate rounded bg-white/5 px-1.5 py-0.5 text-[10px]"
                    >
                        {{ ruleDisplayText }}
                    </span>
                </div>

                <!-- Inline config (expandable for rules with config) -->
                <div v-if="ruleInfo.hasConfig" class="mt-2.5">
                    <!-- remove_brackets -->
                    <div v-if="rule.type === 'remove_brackets'" class="flex gap-1.5">
                        <button
                            @click="updateConfig('keepContent', false)"
                            class="rounded-lg px-3 py-1.5 text-[11px] font-medium transition-all"
                            :class="
                                !rule.config.keepContent
                                    ? 'bg-lava/15 text-lava ring-lava/30 ring-1'
                                    : `text-dim bg-white/5 ring-1 ring-white/5 hover:bg-white/10
                                        hover:text-white`
                            "
                        >
                            <span class="flex items-center gap-1.5">
                                <Icon name="heroicons:trash" size="11" />
                                Remove with content
                            </span>
                        </button>
                        <button
                            @click="updateConfig('keepContent', true)"
                            class="rounded-lg px-3 py-1.5 text-[11px] font-medium transition-all"
                            :class="
                                rule.config.keepContent
                                    ? 'bg-lava/15 text-lava ring-lava/30 ring-1'
                                    : `text-dim bg-white/5 ring-1 ring-white/5 hover:bg-white/10
                                        hover:text-white`
                            "
                        >
                            <span class="flex items-center gap-1.5">
                                <Icon name="heroicons:code-bracket" size="11" />
                                Keep content
                            </span>
                        </button>
                    </div>

                    <!-- regex_remove -->
                    <div v-else-if="rule.type === 'regex_remove'" class="space-y-1.5">
                        <div class="relative">
                            <div
                                class="pointer-events-none absolute inset-y-0 left-0 flex
                                    items-center pl-3"
                            >
                                <span class="text-dim text-[10px] font-medium">/</span>
                            </div>
                            <input
                                :value="rule.config.pattern || ''"
                                @input="
                                    updateConfig(
                                        'pattern',
                                        ($event.target as HTMLInputElement).value,
                                    )
                                "
                                type="text"
                                placeholder="regex pattern"
                                class="bg-void/80 placeholder:text-dim/40 w-full rounded-lg border
                                    py-2 pr-8 pl-5 font-mono text-xs text-white transition-all
                                    focus:ring-1 focus:outline-none"
                                :class="
                                    regexError
                                        ? `border-red-500/50 focus:border-red-500/50
                                            focus:ring-red-500/20`
                                        : 'border-border focus:border-lava/40 focus:ring-lava/20'
                                "
                            />
                            <div
                                class="pointer-events-none absolute inset-y-0 right-0 flex
                                    items-center pr-3"
                            >
                                <span class="text-dim text-[10px] font-medium">/gi</span>
                            </div>
                        </div>
                        <p
                            v-if="regexError"
                            class="flex items-center gap-1 text-[10px] text-red-400"
                        >
                            <Icon name="heroicons:exclamation-triangle" size="10" />
                            {{ regexError }}
                        </p>
                    </div>

                    <!-- text_replace -->
                    <div
                        v-else-if="rule.type === 'text_replace'"
                        class="flex flex-wrap items-center gap-2"
                    >
                        <div class="relative flex-1">
                            <input
                                :value="rule.config.find || ''"
                                @input="
                                    updateConfig('find', ($event.target as HTMLInputElement).value)
                                "
                                type="text"
                                placeholder="Find..."
                                class="border-border bg-void/80 placeholder:text-dim/40
                                    focus:border-lava/40 focus:ring-lava/20 w-full min-w-24
                                    rounded-lg border px-3 py-2 text-xs text-white transition-all
                                    focus:ring-1 focus:outline-none"
                            />
                        </div>
                        <div class="flex h-8 w-8 shrink-0 items-center justify-center">
                            <Icon name="heroicons:arrow-right" size="14" class="text-dim" />
                        </div>
                        <div class="relative flex-1">
                            <input
                                :value="rule.config.replace || ''"
                                @input="
                                    updateConfig(
                                        'replace',
                                        ($event.target as HTMLInputElement).value,
                                    )
                                "
                                type="text"
                                placeholder="Replace with..."
                                class="border-border bg-void/80 placeholder:text-dim/40
                                    focus:border-lava/40 focus:ring-lava/20 w-full min-w-24
                                    rounded-lg border px-3 py-2 text-xs text-white transition-all
                                    focus:ring-1 focus:outline-none"
                            />
                        </div>
                        <label
                            class="border-border flex cursor-pointer items-center gap-1.5 rounded-lg
                                border px-2.5 py-2 text-xs transition-all hover:bg-white/5"
                            :class="
                                rule.config.caseSensitive
                                    ? 'border-lava/30 bg-lava/5 text-lava'
                                    : 'text-dim'
                            "
                        >
                            <input
                                type="checkbox"
                                :checked="rule.config.caseSensitive"
                                @change="
                                    updateConfig(
                                        'caseSensitive',
                                        ($event.target as HTMLInputElement).checked,
                                    )
                                "
                                class="sr-only"
                            />
                            <Icon name="heroicons:language" size="12" />
                            <span class="text-[10px] font-medium">Aa</span>
                        </label>
                    </div>

                    <!-- word_length_filter -->
                    <div
                        v-else-if="rule.type === 'word_length_filter'"
                        class="flex items-center gap-2"
                    >
                        <span class="text-dim text-xs">Remove words shorter than</span>
                        <input
                            :value="rule.config.minLength || 2"
                            @input="
                                updateConfig(
                                    'minLength',
                                    parseInt(($event.target as HTMLInputElement).value) || 2,
                                )
                            "
                            type="number"
                            min="1"
                            max="20"
                            class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                                w-14 rounded-lg border px-2 py-1.5 text-center text-xs text-white
                                tabular-nums transition-all focus:ring-1 focus:outline-none"
                        />
                        <span class="text-dim text-xs">characters</span>
                    </div>

                    <!-- case_normalize -->
                    <div v-else-if="rule.type === 'case_normalize'" class="flex gap-1.5">
                        <button
                            v-for="ct in ['lower', 'upper', 'title'] as const"
                            :key="ct"
                            @click="updateConfig('caseType', ct)"
                            class="rounded-lg px-3 py-1.5 text-[11px] font-medium transition-all"
                            :class="
                                rule.config.caseType === ct
                                    ? 'bg-lava/15 text-lava ring-lava/30 ring-1'
                                    : `text-dim bg-white/5 ring-1 ring-white/5 hover:bg-white/10
                                        hover:text-white`
                            "
                        >
                            <span v-if="ct === 'lower'">lowercase</span>
                            <span v-else-if="ct === 'upper'">UPPERCASE</span>
                            <span v-else>Title Case</span>
                        </button>
                    </div>
                </div>
            </div>

            <!-- Delete button -->
            <button
                @click="$emit('delete')"
                class="text-dim mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-lg
                    opacity-0 transition-all group-hover:opacity-100 hover:bg-red-500/10
                    hover:text-red-400"
            >
                <Icon name="heroicons:trash" size="14" />
            </button>
        </div>

        <!-- Drag indicator line -->
        <div
            v-if="isDragging"
            class="via-lava/50 absolute inset-x-0 bottom-0 h-0.5 bg-gradient-to-r from-transparent
                to-transparent"
        />
    </div>
</template>
