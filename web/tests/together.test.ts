import { describe, expect, test } from 'bun:test';
import {
    bestClockOffset,
    clockSample,
    CLOCK_SAMPLES_KEPT,
    completeEmote,
    driftCorrection,
    emoteFileProblem,
    emoteMap,
    emoteNameFromFile,
    emoteQueryAt,
    expectedPosition,
    inviteLink,
    isCustomEmoteReaction,
    isValidEmoteName,
    itemEnd,
    matchEmotes,
    moveItem,
    pushClockSample,
    reconnectDelay,
    shouldReconnect,
    tokenizeChat,
    TRANSCODE_DRIFT_THRESHOLD,
    type ClockSample,
    type DriftInput,
} from '../app/utils/together';
import { ALL_TOGETHER_EMOJI, QUICK_REACTIONS } from '../app/utils/togetherEmoji';

describe('clock offset', () => {
    test('a sample assumes the server time was read half way', () => {
        // sent at 1000, received at 1100: the server read 5050 at 1050
        expect(clockSample(1000, 5050, 1100)).toEqual({ rtt: 100, offset: 4000 });
    });

    test('the least delayed sample gives the offset', () => {
        let samples: ClockSample[] = [];
        samples = pushClockSample(samples, { rtt: 300, offset: 900 });
        samples = pushClockSample(samples, { rtt: 20, offset: 1000 });
        samples = pushClockSample(samples, { rtt: 150, offset: 1100 });
        expect(bestClockOffset(samples)).toBe(1000);
        expect(bestClockOffset([])).toBe(0);
    });

    test('only the last samples are kept', () => {
        let samples: ClockSample[] = [{ rtt: 1, offset: 42 }];
        for (let i = 0; i < CLOCK_SAMPLES_KEPT; i++) {
            samples = pushClockSample(samples, { rtt: 50 + i, offset: i });
        }
        expect(samples).toHaveLength(CLOCK_SAMPLES_KEPT);
        // the precise but old sample fell out
        expect(bestClockOffset(samples)).toBe(0);
    });
});

describe('expectedPosition', () => {
    const base = { position: 10, server_time: 100000, rate: 1, playing: true, hold: false };

    test('moves with the server time while playing', () => {
        expect(expectedPosition(base, 102500)).toBeCloseTo(12.5);
        expect(expectedPosition({ ...base, rate: 2 }, 101000)).toBeCloseTo(12);
    });

    test('does not move while paused or held', () => {
        expect(expectedPosition({ ...base, playing: false }, 160000)).toBe(10);
        expect(expectedPosition({ ...base, hold: true }, 160000)).toBe(10);
    });

    test('is clamped to the end and never negative', () => {
        expect(expectedPosition(base, 200000, 30)).toBe(30);
        expect(expectedPosition({ ...base, position: -3, playing: false }, 0)).toBe(0);
        // a server time in the future (clock offset error) does not go back
        expect(expectedPosition(base, 99000)).toBe(10);
    });

    test('itemEnd prefers the range end, then the duration', () => {
        expect(itemEnd({ end: 42, duration: 600 })).toBe(42);
        expect(itemEnd({ end: null, duration: 600 })).toBe(600);
        expect(itemEnd({ end: null, duration: 0 })).toBeNull();
        expect(itemEnd(null)).toBeNull();
    });
});

describe('driftCorrection', () => {
    const input = (over: Partial<DriftInput>): DriftInput => ({
        current: 10,
        expected: 10,
        rate: 1,
        threshold: 0.75,
        nudge: true,
        transcoding: false,
        nudging: false,
        ...over,
    });

    test('seeks on a large drift', () => {
        expect(driftCorrection(input({ current: 12 }))).toEqual({ kind: 'seek', position: 10 });
        expect(driftCorrection(input({ current: 9 }))).toEqual({ kind: 'seek', position: 10 });
    });

    test('a converted stream only seeks over the transcode floor', () => {
        expect(driftCorrection(input({ current: 12, transcoding: true })).kind).toBe('rate');
        expect(
            driftCorrection(
                input({ current: 10 + TRANSCODE_DRIFT_THRESHOLD + 0.1, transcoding: true }),
            ).kind,
        ).toBe('seek');
    });

    test('nudges small drifts: behind plays faster, ahead slower', () => {
        const behind = driftCorrection(input({ current: 9.6 }));
        const ahead = driftCorrection(input({ current: 10.4 }));
        expect(behind.kind).toBe('rate');
        expect(ahead.kind).toBe('rate');
        if (behind.kind === 'rate') expect(behind.rate).toBeGreaterThan(1);
        if (ahead.kind === 'rate') expect(ahead.rate).toBeLessThan(1);
    });

    test('a nudge never changes the rate by more than 5%', () => {
        const a = driftCorrection(input({ current: 10.7, rate: 2 }));
        expect(a.kind === 'rate' && a.rate).toBeCloseTo(2 * 0.95);
    });

    test('tiny drifts play at the lobby rate, with hysteresis while nudging', () => {
        expect(driftCorrection(input({ current: 10.08 }))).toEqual({ kind: 'rate', rate: 1 });
        const still = driftCorrection(input({ current: 10.08, nudging: true }));
        expect(still.kind === 'rate' && still.rate).toBeLessThan(1);
        expect(driftCorrection(input({ current: 10.3, nudge: false }))).toEqual({
            kind: 'rate',
            rate: 1,
        });
    });
});

describe('reconnect', () => {
    test('final close codes do not reconnect', () => {
        for (const code of [4001, 4003, 4004, 4009]) expect(shouldReconnect(code)).toBe(false);
        for (const code of [1000, 1001, 1006, 4000, 4008]) expect(shouldReconnect(code)).toBe(true);
    });

    test('the delay doubles from 1 s up to 30 s, with a bounded jitter', () => {
        const mid = () => 0.5;
        expect(reconnectDelay(0, mid)).toBe(1000);
        expect(reconnectDelay(1, mid)).toBe(2000);
        expect(reconnectDelay(4, mid)).toBe(16000);
        expect(reconnectDelay(10, mid)).toBe(30000);
        expect(reconnectDelay(0, () => 0)).toBe(800);
        expect(reconnectDelay(0, () => 1)).toBe(1200);
        expect(reconnectDelay(20, () => 1)).toBe(30000);
    });
});

describe('chat emotes', () => {
    const emotes = emoteMap([
        { name: 'Pog', url: '/emotes/a.png' },
        { name: 'kek_w', url: '/emotes/b.gif' },
    ]);

    test('known :name: become emote tokens, without regard to case', () => {
        expect(tokenizeChat('hi :pog: there :KEK_W:', emotes)).toEqual([
            { kind: 'text', text: 'hi ' },
            { kind: 'emote', name: 'Pog', url: '/emotes/a.png' },
            { kind: 'text', text: ' there ' },
            { kind: 'emote', name: 'kek_w', url: '/emotes/b.gif' },
        ]);
    });

    test('unknown names and markup stay text', () => {
        expect(tokenizeChat(':nope: <b>x</b>', emotes)).toEqual([
            { kind: 'text', text: ':nope: <b>x</b>' },
        ]);
        expect(tokenizeChat('', emotes)).toEqual([]);
        expect(tokenizeChat(':Pog::Pog:', emotes)).toHaveLength(2);
    });

    test('the autocomplete query is the :word before the caret', () => {
        expect(emoteQueryAt('hello :po', 9)).toEqual({ start: 6, query: 'po' });
        expect(emoteQueryAt(':k', 2)).toEqual({ start: 0, query: 'k' });
        expect(emoteQueryAt('time 10:30', 10)).toBeNull();
        expect(emoteQueryAt('hello :', 7)).toBeNull();
        expect(emoteQueryAt('hello :po there', 15)).toBeNull();
    });

    test('matches prefix first, then substring', () => {
        const list = [{ name: 'xpog' }, { name: 'pogger' }, { name: 'Pog' }, { name: 'other' }];
        expect(matchEmotes(list, 'pog').map((e) => e.name)).toEqual(['Pog', 'pogger', 'xpog']);
        expect(matchEmotes(list, 'pog', 1)).toHaveLength(1);
    });

    test('completion replaces the partial and moves the caret after it', () => {
        expect(completeEmote('hey :po rest', 4, 7, 'Pog')).toEqual({
            text: 'hey :Pog:  rest',
            caret: 10,
        });
    });

    test('reaction values', () => {
        expect(isCustomEmoteReaction(':Pog:')).toBe(true);
        expect(isCustomEmoteReaction('🔥')).toBe(false);
        expect(isCustomEmoteReaction(':a:')).toBe(false);
    });
});

describe('emote management helpers', () => {
    test('names', () => {
        expect(isValidEmoteName('Pog_2')).toBe(true);
        expect(isValidEmoteName('a')).toBe(false);
        expect(isValidEmoteName('with space')).toBe(false);
        expect(isValidEmoteName('x'.repeat(33))).toBe(false);
        expect(emoteNameFromFile('Pog Champ!.png')).toBe('Pog_Champ');
        expect(emoteNameFromFile('weird...name.webp')).toBe('weird_name');
    });

    test('files', () => {
        expect(emoteFileProblem({ type: 'image/png', size: 1000 })).toBeNull();
        expect(emoteFileProblem({ type: 'image/jpeg', size: 1000 })).not.toBeNull();
        expect(emoteFileProblem({ type: 'image/gif', size: 10 * 1024 * 1024 })).not.toBeNull();
    });
});

describe('misc', () => {
    test('invite link', () => {
        expect(inviteLink('https://gh.example.com/', 'K7QW2MZP')).toBe(
            'https://gh.example.com/together/join/K7QW2MZP',
        );
    });

    test('moveItem', () => {
        expect(moveItem(['a', 'b', 'c', 'd'], 0, 2)).toEqual(['b', 'c', 'a', 'd']);
        expect(moveItem(['a', 'b', 'c'], 2, 0)).toEqual(['c', 'a', 'b']);
        expect(moveItem(['a', 'b'], 5, 0)).toEqual(['a', 'b']);
    });

    test('the emoji set is curated and unique', () => {
        expect(ALL_TOGETHER_EMOJI.length).toBeGreaterThanOrEqual(150);
        expect(new Set(ALL_TOGETHER_EMOJI).size).toBe(ALL_TOGETHER_EMOJI.length);
        for (const e of QUICK_REACTIONS) expect(ALL_TOGETHER_EMOJI).toContain(e);
    });
});
