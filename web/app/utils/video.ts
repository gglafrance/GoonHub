import type { Video } from '~/types/video';

export function isVideoProcessing(video: Video): boolean {
    return video.processing_status === 'pending' || video.processing_status === 'processing';
}

export function hasVideoError(video: Video): boolean {
    return video.processing_status === 'failed';
}
