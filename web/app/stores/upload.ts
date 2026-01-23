import { defineStore } from 'pinia';

export interface UploadItem {
    id: string;
    file: File;
    title: string;
    progress: number;
    status: 'queued' | 'uploading' | 'completed' | 'failed';
    error?: string;
    videoId?: number;
    xhr?: XMLHttpRequest;
}

const MAX_CONCURRENT = 2;

export const useUploadStore = defineStore('upload', () => {
    const uploads = ref<UploadItem[]>([]);
    const videoStore = useVideoStore();
    const authStore = useAuthStore();

    const activeCount = computed(() =>
        uploads.value.filter((u) => u.status === 'uploading').length,
    );

    const hasActive = computed(() =>
        uploads.value.some((u) => u.status === 'uploading' || u.status === 'queued'),
    );

    function addUpload(file: File, title: string) {
        const id = crypto.randomUUID();
        uploads.value.push({
            id,
            file,
            title,
            progress: 0,
            status: 'queued',
        });
        processQueue();
    }

    function cancelUpload(id: string) {
        const upload = uploads.value.find((u) => u.id === id);
        if (!upload) return;

        if (upload.xhr) {
            upload.xhr.abort();
        }
        uploads.value = uploads.value.filter((u) => u.id !== id);
        processQueue();
    }

    function removeCompleted() {
        uploads.value = uploads.value.filter((u) => u.status !== 'completed');
    }

    function removeUpload(id: string) {
        const upload = uploads.value.find((u) => u.id === id);
        if (upload && upload.xhr) {
            upload.xhr.abort();
        }
        uploads.value = uploads.value.filter((u) => u.id !== id);
    }

    function processQueue() {
        const active = uploads.value.filter((u) => u.status === 'uploading').length;
        const queued = uploads.value.filter((u) => u.status === 'queued');
        const slotsAvailable = MAX_CONCURRENT - active;

        for (let i = 0; i < Math.min(slotsAvailable, queued.length); i++) {
            const item = queued[i];
            if (item) {
                startUpload(item);
            }
        }
    }

    function startUpload(item: UploadItem) {
        item.status = 'uploading';

        const xhr = new XMLHttpRequest();
        item.xhr = xhr;

        const formData = new FormData();
        formData.append('video', item.file);
        if (item.title) {
            formData.append('title', item.title);
        }

        xhr.upload.onprogress = (e: ProgressEvent) => {
            if (e.lengthComputable) {
                item.progress = Math.round((e.loaded / e.total) * 100);
            }
        };

        xhr.onload = () => {
            if (xhr.status >= 200 && xhr.status < 300) {
                try {
                    const response = JSON.parse(xhr.responseText);
                    item.videoId = response.id;
                    item.status = 'completed';
                    item.progress = 100;

                    if (videoStore.currentPage === 1) {
                        videoStore.prependVideo(response);
                    }
                } catch {
                    item.status = 'failed';
                    item.error = 'Failed to parse response';
                }
            } else if (xhr.status === 401) {
                item.status = 'failed';
                item.error = 'Unauthorized';
                authStore.logout();
            } else {
                try {
                    const error = JSON.parse(xhr.responseText);
                    item.status = 'failed';
                    item.error = error.error || 'Upload failed';
                } catch {
                    item.status = 'failed';
                    item.error = `Upload failed (${xhr.status})`;
                }
            }
            item.xhr = undefined;
            processQueue();
        };

        xhr.onerror = () => {
            item.status = 'failed';
            item.error = 'Network error';
            item.xhr = undefined;
            processQueue();
        };

        xhr.onabort = () => {
            item.xhr = undefined;
            processQueue();
        };

        xhr.open('POST', '/api/v1/videos');
        if (authStore.token) {
            xhr.setRequestHeader('Authorization', `Bearer ${authStore.token}`);
        }
        xhr.send(formData);
    }

    return {
        uploads,
        activeCount,
        hasActive,
        addUpload,
        cancelUpload,
        removeCompleted,
        removeUpload,
    };
});
