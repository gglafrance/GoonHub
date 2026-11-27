// Scrolling of the thumbnail scrubber strip: mouse drag with inertia, wheel when the user turned
// it on (vertical wheel scrolls sideways; off, the wheel is left to the page), smooth programmatic scrolls (page buttons, following playback). Touch
// devices keep the native horizontal swipe. The math lives in utils/scrubber.ts.

interface Options {
    // width of the scrollable content, in px
    contentWidth: Ref<number>;
    // the user scrolled the strip (drag, wheel, swipe, scrollbar): playback following pauses
    onUserScroll: () => void;
    // the scrubber wheel setting: off, wheel events are not handled (no preventDefault)
    wheelEnabled: () => boolean;
}

export function useScrubberScroll(el: Ref<HTMLElement | null>, options: Options) {
    const scrollLeft = ref(0);
    const viewWidth = ref(0);
    const dragging = ref(false);

    // scroll position this code set last (a different one in a scroll event comes from the user)
    let expectedLeft = 0;
    // pointer pressed on the strip (mouse drag candidate, or a finger)
    let pressed = false;
    let pointerId = -1;
    let startX = 0;
    let startLeft = 0;
    let samples: DragSample[] = [];
    // a drag just ended: the click that follows must not seek
    let suppressClick = false;

    // animation: smooth scroll towards target, or inertia after a drag
    let target: number | null = null;
    let userTarget = false;
    let velocity = 0;
    let lastFrame = 0;
    let rafId = 0;
    let timerId: ReturnType<typeof setTimeout> | null = null;

    const bounds = () => maxScroll(viewWidth.value, options.contentWidth.value);

    const setLeft = (left: number) => {
        const node = el.value;
        if (!node) return;
        node.scrollLeft = left;
        expectedLeft = node.scrollLeft;
        scrollLeft.value = expectedLeft;
    };

    const cancelFrame = () => {
        if (rafId) cancelAnimationFrame(rafId);
        if (timerId) clearTimeout(timerId);
        rafId = 0;
        timerId = null;
    };

    const stop = () => {
        cancelFrame();
        target = null;
        userTarget = false;
        velocity = 0;
    };

    // rAF, with a timer fallback since rAF does not run in background or throttled tabs
    const schedule = () => {
        if (rafId || timerId) return;
        rafId = requestAnimationFrame(frame);
        timerId = setTimeout(frame, 50);
    };

    function frame() {
        cancelFrame();
        const node = el.value;
        if (!node) return stop();
        const now = performance.now();
        const dt = lastFrame ? Math.min(64, now - lastFrame) : 16;
        lastFrame = now;
        const current = node.scrollLeft;
        if (velocity !== 0) {
            const next = Math.min(bounds(), Math.max(0, current + velocity * dt));
            setLeft(next);
            velocity = next <= 0 || next >= bounds() ? 0 : decayVelocity(velocity, dt);
        } else if (target !== null) {
            const next = smoothScrollStep(current, target);
            setLeft(next);
            if (next === target || Math.abs(node.scrollLeft - current) < 0.01) {
                target = null;
                userTarget = false;
            }
        }
        if (velocity !== 0 || target !== null) schedule();
        else lastFrame = 0;
    }

    // Scrolls to `left` (clamped), smoothly or at once.
    const scrollTo = (left: number, smooth = true, byUser = false) => {
        const clamped = clampScroll(left, viewWidth.value, options.contentWidth.value);
        velocity = 0;
        if (!smooth) {
            stop();
            setLeft(clamped);
            return;
        }
        target = clamped;
        userTarget = byUser;
        schedule();
    };

    // Whether the user is still moving the strip (pressed, gliding, or a wheel scroll running)
    const isBusy = () => pressed || velocity !== 0 || (target !== null && userTarget);

    // A click right after a drag is not a seek
    const consumeDrag = () => {
        const was = suppressClick;
        suppressClick = false;
        return was;
    };

    const onScroll = () => {
        const node = el.value;
        if (!node) return;
        const left = node.scrollLeft;
        if (Math.abs(left - expectedLeft) > 1) {
            // native scroll (touch swipe, scrollbar, keyboard of the browser)
            if (!dragging.value) {
                stop();
                options.onUserScroll();
            }
        }
        expectedLeft = left;
        scrollLeft.value = left;
    };

    const onWheel = (e: WheelEvent) => {
        const delta = scrubberWheelDelta(e, options.wheelEnabled(), viewWidth.value);
        if (delta === 0) return;
        const max = bounds();
        const base = target ?? el.value?.scrollLeft ?? 0;
        // At an end, let the page scroll instead of trapping the wheel
        if ((base <= 0 && delta < 0) || (base >= max && delta > 0)) return;
        e.preventDefault();
        options.onUserScroll();
        scrollTo(base + delta, true, true);
    };

    const onPointerDown = (e: PointerEvent) => {
        if (e.button !== 0) return;
        stop();
        pressed = true;
        pointerId = e.pointerId;
        options.onUserScroll();
        if (e.pointerType !== 'mouse') return; // touch and pen: native scrolling
        startX = e.clientX;
        startLeft = el.value?.scrollLeft ?? 0;
        samples = [{ x: e.clientX, t: performance.now() }];
    };

    const onPointerMove = (e: PointerEvent) => {
        if (!pressed || e.pointerId !== pointerId || e.pointerType !== 'mouse') return;
        const dx = e.clientX - startX;
        if (!dragging.value) {
            if (Math.abs(dx) < SCRUBBER_DRAG_THRESHOLD) return;
            dragging.value = true;
            el.value?.setPointerCapture(e.pointerId);
        }
        e.preventDefault();
        setLeft(clampScroll(startLeft - dx, viewWidth.value, options.contentWidth.value));
        samples.push({ x: e.clientX, t: performance.now() });
        if (samples.length > 12) samples.shift();
    };

    const onPointerUp = (e: PointerEvent) => {
        if (e.pointerId !== pointerId) return;
        pressed = false;
        pointerId = -1;
        options.onUserScroll();
        if (!dragging.value) return;
        dragging.value = false;
        suppressClick = e.type === 'pointerup';
        // the click (if any) is dispatched right after pointerup
        setTimeout(() => {
            suppressClick = false;
        }, 0);
        if (el.value?.hasPointerCapture(e.pointerId)) el.value.releasePointerCapture(e.pointerId);
        samples.push({ x: e.clientX, t: performance.now() });
        velocity = releaseVelocity(samples);
        if (Math.abs(velocity) > SCRUBBER_MIN_VELOCITY) {
            lastFrame = 0;
            schedule();
        } else {
            velocity = 0;
        }
    };

    let resizeObserver: ResizeObserver | null = null;

    const attach = (node: HTMLElement) => {
        node.addEventListener('scroll', onScroll, { passive: true });
        node.addEventListener('wheel', onWheel, { passive: false });
        node.addEventListener('pointerdown', onPointerDown);
        node.addEventListener('pointermove', onPointerMove);
        node.addEventListener('pointerup', onPointerUp);
        node.addEventListener('pointercancel', onPointerUp);
        viewWidth.value = node.clientWidth;
        resizeObserver = new ResizeObserver(() => {
            viewWidth.value = node.clientWidth;
        });
        resizeObserver.observe(node);
        expectedLeft = node.scrollLeft;
        scrollLeft.value = expectedLeft;
    };

    const detach = (node: HTMLElement) => {
        node.removeEventListener('scroll', onScroll);
        node.removeEventListener('wheel', onWheel);
        node.removeEventListener('pointerdown', onPointerDown);
        node.removeEventListener('pointermove', onPointerMove);
        node.removeEventListener('pointerup', onPointerUp);
        node.removeEventListener('pointercancel', onPointerUp);
        resizeObserver?.disconnect();
        resizeObserver = null;
    };

    watch(
        el,
        (node, old) => {
            if (old) detach(old);
            if (node) attach(node);
        },
        { immediate: true },
    );

    onBeforeUnmount(() => {
        stop();
        if (el.value) detach(el.value);
    });

    return { scrollLeft, viewWidth, dragging, scrollTo, stop, isBusy, consumeDrag };
}
