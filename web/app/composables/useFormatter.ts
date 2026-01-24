export const useFormatter = () => {
    const formatDuration = (seconds: number): string => {
        const h = Math.floor(seconds / 3600);
        const m = Math.floor((seconds % 3600) / 60);
        const s = Math.floor(seconds % 60);

        if (h > 0) {
            return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
        }
        return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
    };

    const formatSize = (bytes: number): string => {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    };

    const formatBitRate = (bps: number): string => {
        if (!bps) return '';
        if (bps >= 1_000_000) {
            return `${(bps / 1_000_000).toFixed(1)} Mbps`;
        }
        if (bps >= 1_000) {
            return `${(bps / 1_000).toFixed(0)} Kbps`;
        }
        return `${bps} bps`;
    };

    const formatFrameRate = (fps: number): string => {
        if (!fps) return '';
        if (fps === Math.floor(fps)) {
            return `${fps} fps`;
        }
        return `${fps.toFixed(3)} fps`;
    };

    return {
        formatDuration,
        formatSize,
        formatBitRate,
        formatFrameRate,
    };
};
