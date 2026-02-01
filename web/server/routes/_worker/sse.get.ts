import { readFileSync } from 'fs';
import { join } from 'path';

let workerCode: string | null = null;

export default defineEventHandler((event) => {
    setResponseHeader(event, 'Content-Type', 'application/javascript; charset=utf-8');
    setResponseHeader(event, 'Cache-Control', 'no-cache, no-store, must-revalidate');
    setResponseHeader(event, 'X-Content-Type-Options', 'nosniff');

    if (!workerCode) {
        // In dev: read from server/workers/
        // In prod: file will be in the same relative location
        const workerPath = join(process.cwd(), 'server/workers/sse-worker.js');
        workerCode = readFileSync(workerPath, 'utf-8');
    }

    return workerCode;
});
