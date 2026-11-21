<script setup lang="ts">
import type { PlayerGestureIndicator } from '~/composables/usePlayerGestures';

// Feedback of the player touch gestures, teleported into the video.js element so that it also
// shows in fullscreen. It never takes pointer events.
const props = defineProps<{
    indicator: PlayerGestureIndicator | null;
}>();

const level = computed(() => {
    const i = props.indicator;
    if (!i || (i.kind !== 'volume' && i.kind !== 'brightness')) return null;
    const max = i.kind === 'brightness' ? GESTURE_BRIGHTNESS_MAX : 1;
    return {
        kind: i.kind,
        percent: Math.round(i.level * 100),
        fill: Math.round((i.level / max) * 100),
        icon:
            i.kind === 'brightness'
                ? 'heroicons:sun'
                : i.level === 0
                  ? 'heroicons:speaker-x-mark'
                  : 'heroicons:speaker-wave',
    };
});

const scrubThumbStyle = computed(() => {
    const i = props.indicator;
    if (i?.kind !== 'scrub' || !i.cue) return null;
    return {
        width: `${i.cue.w}px`,
        height: `${i.cue.h}px`,
        backgroundImage: `url("${i.cue.url}")`,
        backgroundPosition: `-${i.cue.x}px -${i.cue.y}px`,
    };
});
</script>

<template>
    <div class="vjs-gesture-overlay" aria-hidden="true">
        <!-- Double-tap seek: ripple on the tapped side with the accumulated amount -->
        <div
            v-if="indicator?.kind === 'seek'"
            class="vjs-gesture-seek"
            :class="
                indicator.side === 'left' ? 'vjs-gesture-seek--left' : 'vjs-gesture-seek--right'
            "
        >
            <span :key="indicator.id" class="vjs-gesture-ripple" />
            <div class="vjs-gesture-seek-label">
                <Icon
                    :name="indicator.side === 'left' ? 'heroicons:backward' : 'heroicons:forward'"
                    size="22"
                />
                <span>{{ formatGestureSeek(indicator.amount) }}</span>
            </div>
        </div>

        <!-- Volume / brightness swipe -->
        <div
            v-else-if="level"
            class="vjs-gesture-level"
            :class="
                level.kind === 'volume' ? 'vjs-gesture-level--right' : 'vjs-gesture-level--left'
            "
        >
            <Icon :name="level.icon" size="16" />
            <div class="vjs-gesture-level-track">
                <div class="vjs-gesture-level-fill" :style="{ height: `${level.fill}%` }" />
            </div>
            <span class="vjs-gesture-level-value">{{ level.percent }}%</span>
        </div>

        <!-- Horizontal scrub: sprite preview, target time and offset -->
        <div v-else-if="indicator?.kind === 'scrub'" class="vjs-gesture-pill vjs-gesture-scrub">
            <div v-if="scrubThumbStyle" class="vjs-gesture-thumb" :style="scrubThumbStyle" />
            <div class="vjs-gesture-scrub-time">
                <span class="text-white">{{ formatGestureClock(indicator.time) }}</span>
                <span v-if="indicator.duration" class="text-white/40">
                    / {{ formatGestureClock(indicator.duration) }}
                </span>
            </div>
            <div class="vjs-gesture-scrub-delta">{{ formatGestureOffset(indicator.delta) }}</div>
        </div>

        <!-- Long-press speed -->
        <div v-else-if="indicator?.kind === 'speed'" class="vjs-gesture-pill vjs-gesture-speed">
            <Icon name="heroicons:forward" size="14" />
            <span>{{ indicator.rate }}x</span>
        </div>
    </div>
</template>

<style scoped>
.vjs-gesture-overlay {
    position: absolute;
    inset: 0;
    z-index: 3;
    pointer-events: none;
    overflow: hidden;
    font-family: 'Inter', system-ui, sans-serif;
}

/* Double-tap seek */
.vjs-gesture-seek {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 38%;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    background: rgba(255, 255, 255, 0.06);
}

.vjs-gesture-seek--left {
    left: 0;
    border-radius: 0 50% 50% 0 / 0 50% 50% 0;
}

.vjs-gesture-seek--right {
    right: 0;
    border-radius: 50% 0 0 50% / 50% 0 0 50%;
}

.vjs-gesture-ripple {
    position: absolute;
    top: 50%;
    left: 50%;
    width: 120px;
    height: 120px;
    margin: -60px 0 0 -60px;
    border-radius: 50%;
    background: rgba(255, 77, 77, 0.25);
    animation: vjs-gesture-ripple 0.55s ease-out forwards;
}

@keyframes vjs-gesture-ripple {
    from {
        transform: scale(0.2);
        opacity: 1;
    }
    to {
        transform: scale(2.4);
        opacity: 0;
    }
}

.vjs-gesture-seek-label {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    color: #ffffff;
    font-family: 'JetBrains Mono', 'SF Mono', monospace;
    font-size: 13px;
    font-weight: 600;
    text-shadow: 0 1px 6px rgba(0, 0, 0, 0.6);
}

/* Volume / brightness */
.vjs-gesture-level {
    position: absolute;
    top: 50%;
    transform: translateY(-50%);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 10px 8px;
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 14px;
    background: rgba(8, 8, 8, 0.7);
    backdrop-filter: blur(12px);
    color: #ffffff;
}

.vjs-gesture-level--left {
    left: 24px;
}

.vjs-gesture-level--right {
    right: 24px;
}

.vjs-gesture-level-track {
    position: relative;
    width: 4px;
    height: 96px;
    border-radius: 2px;
    background: rgba(255, 255, 255, 0.15);
    overflow: hidden;
}

.vjs-gesture-level-fill {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    border-radius: 2px;
    background: linear-gradient(0deg, #ff4d4d, #ff6b6b);
    box-shadow: 0 0 8px rgba(255, 77, 77, 0.5);
}

.vjs-gesture-level-value {
    font-family: 'JetBrains Mono', 'SF Mono', monospace;
    font-size: 10px;
    color: rgba(255, 255, 255, 0.7);
}

/* Scrub and speed pills */
.vjs-gesture-pill {
    position: absolute;
    top: 16px;
    left: 50%;
    transform: translateX(-50%);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    background: rgba(8, 8, 8, 0.75);
    backdrop-filter: blur(12px);
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
    color: #ffffff;
}

.vjs-gesture-scrub {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    padding: 6px;
}

.vjs-gesture-thumb {
    border: 1px solid rgba(255, 77, 77, 0.3);
    border-radius: 8px;
    background-repeat: no-repeat;
    background-color: #0a0a0a;
}

.vjs-gesture-scrub-time {
    padding: 0 6px;
    font-family: 'JetBrains Mono', 'SF Mono', monospace;
    font-size: 12px;
    font-weight: 500;
}

.vjs-gesture-scrub-delta {
    padding: 0 6px 2px;
    font-family: 'JetBrains Mono', 'SF Mono', monospace;
    font-size: 10px;
    color: #ff4d4d;
}

.vjs-gesture-speed {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 12px;
    font-family: 'JetBrains Mono', 'SF Mono', monospace;
    font-size: 12px;
    font-weight: 600;
    color: #ff4d4d;
}
</style>
