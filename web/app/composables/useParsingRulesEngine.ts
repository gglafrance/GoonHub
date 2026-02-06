import type { ParsingRule, ParsingPreset, ParsingRulesSettings } from '~/types/parsing-rules';

/**
 * Escape special regex characters in a string
 */
function escapeRegex(str: string): string {
    return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * Core parsing rules engine for transforming filenames.
 * Apply user-defined rules to clean filenames before PornDB search.
 */
export function useParsingRulesEngine() {
    /**
     * Apply a single rule to input text
     */
    function applyRule(input: string, rule: ParsingRule): string {
        if (!rule.enabled) return input;

        switch (rule.type) {
            case 'remove_brackets':
                if (rule.config.keepContent) {
                    // Only remove brackets, keep content inside
                    return input
                        .replace(/\[([^\]]*)\]/g, '$1')
                        .replace(/\(([^)]*)\)/g, '$1')
                        .replace(/\{([^}]*)\}/g, '$1');
                }
                // Remove brackets and their content (default)
                return input
                    .replace(/\[[^\]]*\]/g, '')
                    .replace(/\([^)]*\)/g, '')
                    .replace(/\{[^}]*\}/g, '');

            case 'remove_numbers':
                return input.replace(/\b\d+\b/g, '');

            case 'remove_years': {
                const currentYear = new Date().getFullYear();
                const maxYear = currentYear + 5;
                // Match years from 1990 to maxYear
                const yearRegex = new RegExp(
                    `\\b(199[0-9]|20[0-${Math.floor(maxYear / 10) % 10}][0-9]|20${Math.floor(maxYear / 10) % 10}[0-${maxYear % 10}])\\b`,
                    'g',
                );
                return input.replace(yearRegex, '');
            }

            case 'remove_special_chars':
                return input.replace(/[_.\-]+/g, ' ');

            case 'remove_stopwords': {
                const stopwords = ['the', 'a', 'an', 'of', 'in', 'on', 'at', 'for', 'to', 'with'];
                const regex = new RegExp(`\\b(${stopwords.join('|')})\\b`, 'gi');
                return input.replace(regex, '');
            }

            case 'remove_duplicates': {
                const words = input.split(/\s+/);
                const seen = new Set<string>();
                return words
                    .filter((w) => {
                        const lower = w.toLowerCase();
                        if (seen.has(lower)) return false;
                        seen.add(lower);
                        return true;
                    })
                    .join(' ');
            }

            case 'regex_remove':
                if (!rule.config.pattern) return input;
                try {
                    return input.replace(new RegExp(rule.config.pattern, 'gi'), '');
                } catch {
                    return input;
                }

            case 'text_replace':
                if (!rule.config.find) return input;
                try {
                    const flags = rule.config.caseSensitive ? 'g' : 'gi';
                    return input.replace(
                        new RegExp(escapeRegex(rule.config.find), flags),
                        rule.config.replace || '',
                    );
                } catch {
                    return input;
                }

            case 'word_length_filter': {
                const minLen = rule.config.minLength || 2;
                // Match sequences of letters only, remove those shorter than minLen
                // This preserves punctuation, numbers, and spaces while filtering short words
                return input.replace(/[a-zA-Z]+/g, (match) =>
                    match.length >= minLen ? match : '',
                );
            }

            case 'case_normalize':
                switch (rule.config.caseType) {
                    case 'lower':
                        return input.toLowerCase();
                    case 'upper':
                        return input.toUpperCase();
                    case 'title':
                        return input.replace(/\b\w/g, (c) => c.toUpperCase());
                    default:
                        return input;
                }

            default:
                return input;
        }
    }

    /**
     * Apply all rules to a filename.
     * Always removes file extension first, then applies rules in order.
     */
    function applyRules(filename: string, rules: ParsingRule[]): string {
        // 1. Always remove file extension first
        let result = filename.replace(/\.[^/.]+$/, '');

        // 2. Sort rules by order, filter enabled
        const activeRules = rules.filter((r) => r.enabled).sort((a, b) => a.order - b.order);

        // 3. Apply each rule sequentially
        for (const rule of activeRules) {
            result = applyRule(result, rule);
        }

        // 4. Final cleanup (collapse whitespace, trim)
        result = result.replace(/\s+/g, ' ').trim();

        // 5. If empty, return original (minus extension)
        if (!result) {
            return filename.replace(/\.[^/.]+$/, '');
        }

        return result;
    }

    /**
     * Validate a regex pattern
     */
    function validateRegex(pattern: string): { valid: boolean; error?: string } {
        try {
            new RegExp(pattern);
            return { valid: true };
        } catch (e) {
            return { valid: false, error: e instanceof Error ? e.message : 'Invalid regex' };
        }
    }

    /**
     * Get rules from a preset ID
     */
    function getRulesFromPreset(
        settings: ParsingRulesSettings | null,
        presetId: string | null,
    ): ParsingRule[] {
        if (!settings || !presetId) return [];
        const preset = settings.presets.find((p) => p.id === presetId);
        return preset ? preset.rules : [];
    }

    /**
     * Generate a unique ID for new rules/presets
     */
    function generateId(): string {
        return crypto.randomUUID();
    }

    /**
     * Create built-in presets
     */
    function getBuiltInPresets(): ParsingPreset[] {
        return [
            {
                id: 'builtin-basic',
                name: 'Basic Cleanup',
                isBuiltIn: true,
                rules: [
                    {
                        id: 'basic-1',
                        type: 'remove_brackets',
                        enabled: true,
                        order: 0,
                        config: {},
                    },
                    {
                        id: 'basic-2',
                        type: 'remove_special_chars',
                        enabled: true,
                        order: 1,
                        config: {},
                    },
                    {
                        id: 'basic-3',
                        type: 'remove_duplicates',
                        enabled: true,
                        order: 2,
                        config: {},
                    },
                    {
                        id: 'basic-4',
                        type: 'case_normalize',
                        enabled: true,
                        order: 3,
                        config: { caseType: 'lower' },
                    },
                ],
            },
            {
                id: 'builtin-vr',
                name: 'VR Scene Cleanup',
                isBuiltIn: true,
                rules: [
                    { id: 'vr-1', type: 'remove_brackets', enabled: true, order: 0, config: {} },
                    { id: 'vr-2', type: 'remove_years', enabled: true, order: 1, config: {} },
                    {
                        id: 'vr-3',
                        type: 'remove_special_chars',
                        enabled: true,
                        order: 2,
                        config: {},
                    },
                    {
                        id: 'vr-4',
                        type: 'text_replace',
                        enabled: true,
                        order: 3,
                        config: { find: 'VR2Normal', replace: '', caseSensitive: false },
                    },
                    {
                        id: 'vr-5',
                        type: 'text_replace',
                        enabled: true,
                        order: 4,
                        config: { find: '6K', replace: '', caseSensitive: false },
                    },
                    {
                        id: 'vr-6',
                        type: 'text_replace',
                        enabled: true,
                        order: 5,
                        config: { find: '5K', replace: '', caseSensitive: false },
                    },
                    { id: 'vr-7', type: 'remove_duplicates', enabled: true, order: 6, config: {} },
                    {
                        id: 'vr-8',
                        type: 'case_normalize',
                        enabled: true,
                        order: 7,
                        config: { caseType: 'lower' },
                    },
                ],
            },
            {
                id: 'builtin-release',
                name: 'Release Group Cleanup',
                isBuiltIn: true,
                rules: [
                    {
                        id: 'release-1',
                        type: 'remove_brackets',
                        enabled: true,
                        order: 0,
                        config: {},
                    },
                    {
                        id: 'release-2',
                        type: 'regex_remove',
                        enabled: true,
                        order: 1,
                        config: { pattern: '\\b(rarbg|yts|yify|sparks|geckos|megusta|fgt)\\b' },
                    },
                    {
                        id: 'release-3',
                        type: 'regex_remove',
                        enabled: true,
                        order: 2,
                        config: { pattern: '\\b(x264|x265|h264|h265|hevc|avc)\\b' },
                    },
                    {
                        id: 'release-4',
                        type: 'regex_remove',
                        enabled: true,
                        order: 3,
                        config: { pattern: '\\b(2160p|1080p|720p|480p|4k|uhd|fhd|hd)\\b' },
                    },
                    {
                        id: 'release-5',
                        type: 'remove_special_chars',
                        enabled: true,
                        order: 4,
                        config: {},
                    },
                    {
                        id: 'release-6',
                        type: 'remove_duplicates',
                        enabled: true,
                        order: 5,
                        config: {},
                    },
                ],
            },
        ];
    }

    /**
     * Get all available presets: stored user presets (which may include modified built-ins)
     * merged with any hardcoded built-in presets not yet in the store.
     * This ensures user modifications are respected while still showing unmodified built-ins.
     */
    function getAllPresets(settings: ParsingRulesSettings | null): ParsingPreset[] {
        const builtIn = getBuiltInPresets();
        const stored = settings?.presets || [];

        // Start with stored presets (includes user-modified built-ins and custom presets)
        const result = [...stored];

        // Add any hardcoded built-in presets not yet saved to the store
        for (const preset of builtIn) {
            if (!result.find((p) => p.id === preset.id)) {
                result.push(preset);
            }
        }

        return result;
    }

    return {
        applyRules,
        applyRule,
        validateRegex,
        getRulesFromPreset,
        generateId,
        getBuiltInPresets,
        getAllPresets,
    };
}
