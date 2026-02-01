import type { Scene, SceneListItem } from '~/types/scene';

export function isSceneProcessing(scene: SceneListItem): boolean {
    return scene.processing_status === 'pending' || scene.processing_status === 'processing';
}

export function hasSceneError(scene: Scene): boolean {
    return scene.processing_status === 'failed';
}
