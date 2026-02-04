export type ParsingRuleType =
    | 'remove_brackets' // Remove [...], (...), {...}
    | 'remove_numbers' // Remove standalone numbers
    | 'remove_years' // Remove 1990-2031 patterns
    | 'remove_special_chars' // Replace _, ., - with spaces
    | 'remove_stopwords' // Remove: the, a, an, of, in, on, at, for, to, with
    | 'remove_duplicates' // Remove duplicate words
    | 'regex_remove' // Remove matches of custom regex
    | 'text_replace' // Replace text (with case toggle)
    | 'word_length_filter' // Remove words shorter than N chars
    | 'case_normalize'; // Convert to lower/upper/title case

export interface ParsingRuleConfig {
    // remove_brackets
    keepContent?: boolean; // If true, only remove brackets but keep content inside
    // regex_remove
    pattern?: string;
    // text_replace
    find?: string;
    replace?: string;
    caseSensitive?: boolean;
    // word_length_filter
    minLength?: number;
    // case_normalize
    caseType?: 'lower' | 'upper' | 'title';
}

export interface ParsingRule {
    id: string; // UUID for ordering
    type: ParsingRuleType;
    enabled: boolean;
    order: number;
    config: ParsingRuleConfig;
}

export interface ParsingPreset {
    id: string;
    name: string;
    isBuiltIn: boolean;
    rules: ParsingRule[];
}

export interface ParsingRulesSettings {
    presets: ParsingPreset[];
    activePresetId: string | null;
}

// Rule type display names and descriptions
export const RULE_TYPE_INFO: Record<
    ParsingRuleType,
    { label: string; description: string; hasConfig: boolean }
> = {
    remove_brackets: {
        label: 'Remove Brackets',
        description: 'Remove [...], (...), and {...} brackets',
        hasConfig: true,
    },
    remove_numbers: {
        label: 'Remove Numbers',
        description: 'Remove standalone numbers',
        hasConfig: false,
    },
    remove_years: {
        label: 'Remove Years',
        description: 'Remove year patterns (1990-2031)',
        hasConfig: false,
    },
    remove_special_chars: {
        label: 'Remove Special Chars',
        description: 'Replace _, ., - with spaces',
        hasConfig: false,
    },
    remove_stopwords: {
        label: 'Remove Stopwords',
        description: 'Remove: the, a, an, of, in, on, at, for, to, with',
        hasConfig: false,
    },
    remove_duplicates: {
        label: 'Remove Duplicates',
        description: 'Remove duplicate words',
        hasConfig: false,
    },
    regex_remove: {
        label: 'Regex Remove',
        description: 'Remove matches of custom regex pattern',
        hasConfig: true,
    },
    text_replace: {
        label: 'Text Replace',
        description: 'Replace text with another value',
        hasConfig: true,
    },
    word_length_filter: {
        label: 'Word Length Filter',
        description: 'Remove words shorter than N characters',
        hasConfig: true,
    },
    case_normalize: {
        label: 'Normalize Case',
        description: 'Convert text to lowercase, uppercase, or title case',
        hasConfig: true,
    },
};
