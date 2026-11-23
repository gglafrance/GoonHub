import { describe, expect, test } from 'bun:test';
import { readResponseBody } from '../app/utils/apiResponse';

describe('readResponseBody', () => {
    test('204 No Content gives null (DELETE endpoints)', async () => {
        expect(await readResponseBody(new Response(null, { status: 204 }))).toBeNull();
    });

    test('an empty 200 body gives null', async () => {
        expect(await readResponseBody(new Response('', { status: 200 }))).toBeNull();
    });

    test('a JSON body is parsed', async () => {
        const res = new Response(JSON.stringify({ data: [1, 2] }), { status: 200 });
        expect(await readResponseBody(res)).toEqual({ data: [1, 2] });
    });

    test('invalid JSON still throws', async () => {
        await expect(readResponseBody(new Response('<html>', { status: 200 }))).rejects.toThrow();
    });
});
