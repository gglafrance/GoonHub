import { describe, expect, test } from 'bun:test';
import {
    AUTO_NEXT_MODES,
    autoNextMode,
    byeReasonText,
    isSelf,
    memberKey,
    memberTarget,
} from '../app/utils/together';

describe('anonymous members', () => {
    const named = { user_id: 3, anonymous: false, anon_number: null };
    // as seen by a regular member: no user id
    const anon = { user_id: null, anonymous: true, anon_number: 2 };
    // as seen by a moderator: real id and the number
    const anonForModerator = { user_id: 7, anonymous: true, anon_number: 2 };

    test('memberKey is stable with or without a user id', () => {
        expect(memberKey(named)).toBe('u3');
        expect(memberKey(anon)).toBe('a2');
        expect(memberKey(anonForModerator)).toBe('u7');
    });

    test('commands name anonymous members by number', () => {
        expect(memberTarget(named)).toEqual({ user_id: 3 });
        expect(memberTarget(anon)).toEqual({ anon: 2 });
        expect(memberTarget(anonForModerator)).toEqual({ anon: 2 });
        expect(memberTarget({ user_id: null, anonymous: false, anon_number: null })).toBeNull();
    });

    test('isSelf matches by user id, or by number when anonymous', () => {
        const meNamed = { user_id: 3, anon_number: null };
        const meAnon = { user_id: 7, anon_number: 2 };
        expect(isSelf(3, null, meNamed)).toBe(true);
        expect(isSelf(null, 2, meNamed)).toBe(false);
        expect(isSelf(null, 2, meAnon)).toBe(true);
        expect(isSelf(7, 2, meAnon)).toBe(true);
        expect(isSelf(null, 1, meAnon)).toBe(false);
        expect(isSelf(null, null, meAnon)).toBe(false);
        expect(isSelf(3, null, null)).toBe(false);
    });
});

describe('auto-next and closing', () => {
    test('every mode has an entry, unknown values fall back to next', () => {
        expect(AUTO_NEXT_MODES.map((m) => m.value).sort()).toEqual(
            ['next', 'repeat_one', 'repeat_queue', 'stop'].sort(),
        );
        expect(autoNextMode('repeat_one').label).toBe('Repeat item');
        expect(autoNextMode('shuffle').value).toBe('next');
        expect(autoNextMode(null).value).toBe('next');
    });

    test('bye texts', () => {
        expect(byeReasonText('inactive')).toContain('inactivity');
        expect(byeReasonText('closed')).toBe('This lobby was closed.');
        expect(byeReasonText('kicked')).toContain('kicked');
    });
});
