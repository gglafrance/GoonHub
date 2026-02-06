/**
 * Composable for job-related formatting utilities.
 */
export const useJobFormatting = () => {
    const formatDuration = (startedAt: string, completedAt?: string): string => {
        const start = new Date(startedAt).getTime();
        const end = completedAt ? new Date(completedAt).getTime() : Date.now();
        const ms = end - start;

        if (ms < 1000) return `${ms}ms`;
        const seconds = Math.floor(ms / 1000);
        if (seconds < 60) return `${seconds}s`;
        const minutes = Math.floor(seconds / 60);
        const remainingSec = seconds % 60;
        return `${minutes}m ${remainingSec}s`;
    };

    const formatTime = (dateStr: string): string => {
        const d = new Date(dateStr);
        return d.toLocaleDateString('en-US', {
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
        });
    };

    const statusClass = (status: string): string => {
        switch (status) {
            case 'running':
                return 'bg-amber-500/15 text-amber-400 border-amber-500/30';
            case 'completed':
                return 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30';
            case 'failed':
                return 'bg-lava/15 text-lava border-lava/30';
            case 'cancelled':
                return 'bg-white/5 text-dim border-white/10';
            case 'timed_out':
                return 'bg-orange-500/15 text-orange-400 border-orange-500/30';
            default:
                return 'bg-white/5 text-dim border-white/10';
        }
    };

    const phaseLabel = (phase: string): string => {
        switch (phase) {
            case 'metadata':
                return 'Metadata';
            case 'thumbnail':
                return 'Thumbnail';
            case 'sprites':
                return 'Sprites';
            case 'animated_thumbnails':
                return 'Previews';
            default:
                return phase;
        }
    };

    const phaseIcon = (phase: string): string => {
        switch (phase) {
            case 'metadata':
                return 'heroicons:document-text';
            case 'thumbnail':
                return 'heroicons:photo';
            case 'sprites':
                return 'heroicons:squares-2x2';
            case 'animated_thumbnails':
                return 'heroicons:film';
            default:
                return 'heroicons:cog-6-tooth';
        }
    };

    return {
        formatDuration,
        formatTime,
        statusClass,
        phaseLabel,
        phaseIcon,
    };
};
