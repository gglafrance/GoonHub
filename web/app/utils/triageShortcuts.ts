// Keyboard shortcuts of the triage page, shown in the page's hint bar and in the keyboard
// shortcuts modal.
export interface TriageShortcut {
    keys: string[];
    description: string;
    // Short label for the hint bar
    hint: string;
}

export const TRIAGE_SHORTCUTS: TriageShortcut[] = [
    { keys: ['1-5'], description: 'Rate the scene', hint: 'Rate' },
    { keys: ['0'], description: 'Clear the rating', hint: 'Clear rating' },
    { keys: ['L'], description: 'Like / unlike', hint: 'Like' },
    { keys: ['T'], description: 'Add tags', hint: 'Tag' },
    { keys: ['A'], description: 'Add actors', hint: 'Actor' },
    { keys: ['S'], description: 'Set the studio', hint: 'Studio' },
    { keys: ['Del', '⌫'], description: 'Move to trash (undo in the toast)', hint: 'Trash' },
    { keys: ['→', 'Space', 'Enter'], description: 'Next scene', hint: 'Next' },
    { keys: ['←'], description: 'Previous scene', hint: 'Previous' },
    { keys: ['P'], description: 'Play / Pause', hint: 'Play' },
    { keys: ['Esc'], description: 'Leave a field, back to the keys', hint: 'Leave field' },
    { keys: ['?'], description: 'Show the shortcuts', hint: 'Shortcuts' },
];
