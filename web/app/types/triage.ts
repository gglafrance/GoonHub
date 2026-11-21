// Suggestion of a triage autocomplete field (tag, actor or studio)
export interface TriageOption {
    id: number;
    name: string;
    // Secondary text, e.g. the number of scenes
    hint?: string;
}
