import type { PornDBScene } from './porndb';
import type { SceneMatchInfo } from './explorer';

export type { SceneMatchInfo } from './explorer';

export interface ConfidenceBreakdown {
    titleScore: number; // 0-30
    actorScore: number; // 0-30
    studioScore: number; // 0-20
    durationScore: number; // 0-20
    total: number; // 0-100
}

export type MatchStatus =
    | 'pending'
    | 'searching'
    | 'matched'
    | 'no-match'
    | 'skipped'
    | 'removed'
    | 'applying'
    | 'applied'
    | 'failed';

export interface BulkMatchResult {
    sceneId: number;
    localScene: SceneMatchInfo;
    match: PornDBScene | null;
    confidence: ConfidenceBreakdown | null;
    status: MatchStatus;
    error?: string;
}

export type ApplyPhase = 'idle' | 'applying' | 'done';
