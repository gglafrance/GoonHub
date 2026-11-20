import type videojs from 'video.js';

type Player = ReturnType<typeof videojs>;

export const useABLoop = (player: Ref<Player | null> | ShallowRef<Player | null>) => {
    const enabled = ref(false);
    const pointA = ref<number | null>(null);
    const pointB = ref<number | null>(null);

    // Player duration as a reactive value, so that the progress bar overlay is placed once the
    // metadata is loaded (points can be set before, e.g. when opening a marker link).
    const duration = ref(0);
    const updateDuration = () => {
        duration.value = player.value?.duration() ?? 0;
    };
    watch(
        player,
        (p, previous) => {
            previous?.off('durationchange', updateDuration);
            previous?.off('loadedmetadata', updateDuration);
            if (!p) return;
            p.on('durationchange', updateDuration);
            p.on('loadedmetadata', updateDuration);
            updateDuration();
        },
        { immediate: true },
    );

    const formatTime = (seconds: number | null): string => {
        if (seconds === null) return '--:--';
        const h = Math.floor(seconds / 3600);
        const m = Math.floor((seconds % 3600) / 60);
        const s = Math.floor(seconds % 60);
        const ms = Math.floor((seconds % 1) * 100)
            .toString()
            .padStart(2, '0');
        if (h > 0) {
            return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}.${ms}`;
        }
        return `${m}:${s.toString().padStart(2, '0')}.${ms}`;
    };

    const formattedA = computed(() => formatTime(pointA.value));
    const formattedB = computed(() => formatTime(pointB.value));

    const onTimeUpdate = () => {
        if (!enabled.value || pointA.value === null || pointB.value === null) return;
        const p = player.value;
        if (!p) return;
        const currentTime = p.currentTime() ?? 0;
        if (currentTime >= pointB.value) {
            p.currentTime(pointA.value);
        }
    };

    const bindTimeUpdate = () => {
        const p = player.value;
        if (p) p.on('timeupdate', onTimeUpdate);
    };

    const unbindTimeUpdate = () => {
        const p = player.value;
        if (p) p.off('timeupdate', onTimeUpdate);
    };

    // Set start point to current player time (left-click on start button)
    // Rejected if current time is at or after point B
    const setStart = () => {
        const p = player.value;
        if (!p) return;
        const currentTime = p.currentTime() ?? 0;
        if (pointB.value !== null && currentTime >= pointB.value) return;
        pointA.value = currentTime;
        // Auto-enable if both points are set
        if (pointB.value !== null && !enabled.value) {
            enabled.value = true;
            bindTimeUpdate();
        }
    };

    // Set end point to current player time (left-click on end button)
    // Rejected if current time is at or before point A
    const setEnd = () => {
        const p = player.value;
        if (!p) return;
        const currentTime = p.currentTime() ?? 0;
        if (pointA.value !== null && currentTime <= pointA.value) return;
        pointB.value = currentTime;
        // Auto-enable if both points are set
        if (pointA.value !== null && !enabled.value) {
            enabled.value = true;
            bindTimeUpdate();
        }
    };

    // Set both points at once (used for marker ranges). With enable, looping starts right away;
    // otherwise the points are set and the loop stays off until toggled.
    const setRange = (start: number, end: number, enable = true) => {
        if (end <= start) return;
        pointA.value = start;
        pointB.value = end;
        if (enable && !enabled.value) {
            enabled.value = true;
            bindTimeUpdate();
        } else if (!enable && enabled.value) {
            enabled.value = false;
            unbindTimeUpdate();
        }
    };

    // Seek to start point (right-click on start button)
    const seekToStart = () => {
        const p = player.value;
        if (p && pointA.value !== null) {
            p.currentTime(pointA.value);
        }
    };

    // Seek to end point (right-click on end button)
    const seekToEnd = () => {
        const p = player.value;
        if (p && pointB.value !== null) {
            p.currentTime(pointB.value);
        }
    };

    // Toggle looping on/off (the loop enable button)
    const toggleEnabled = () => {
        if (enabled.value) {
            enabled.value = false;
            unbindTimeUpdate();
        } else if (pointA.value !== null && pointB.value !== null) {
            enabled.value = true;
            bindTimeUpdate();
        }
    };

    // 3-press toggle for keyboard shortcut (O key): set A -> set B + enable -> clear
    const toggle = () => {
        const p = player.value;
        if (!p) return;

        if (pointA.value === null) {
            setStart();
        } else if (pointB.value === null) {
            setEnd();
        } else {
            clear();
        }
    };

    const clear = () => {
        unbindTimeUpdate();
        enabled.value = false;
        pointA.value = null;
        pointB.value = null;
    };

    // Percentage positions for progress bar overlay
    const progressA = computed(() => {
        if (pointA.value === null || duration.value <= 0) return null;
        return (pointA.value / duration.value) * 100;
    });

    const progressB = computed(() => {
        if (pointB.value === null || duration.value <= 0) return null;
        return (pointB.value / duration.value) * 100;
    });

    const hasPoints = computed(() => pointA.value !== null || pointB.value !== null);

    onBeforeUnmount(() => {
        clear();
    });

    return {
        enabled,
        pointA,
        pointB,
        formattedA,
        formattedB,
        progressA,
        progressB,
        hasPoints,
        setStart,
        setEnd,
        setRange,
        seekToStart,
        seekToEnd,
        toggleEnabled,
        toggle,
        clear,
    };
};
