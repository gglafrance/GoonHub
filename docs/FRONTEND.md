# Frontend Architecture

This document describes the internal architecture of the GoonHub Nuxt 4 frontend.

## Tech Stack

| Technology | Version | Purpose |
|-----------|---------|---------|
| Nuxt | 4.2 | Meta-framework (SPA mode, SSR disabled) |
| Vue | 3.5 | UI framework (Composition API) |
| Pinia | 3.0 | State management |
| Tailwind CSS | 4.1 | Utility-first styling |
| TypeScript | - | Type safety |
| video.js | 8.23 | Video player |
| media-chrome | 4.17 | Custom media controls |
| @nuxt/icon | - | Icon components (Heroicons, etc.) |

The frontend compiles to static assets (`web/dist/`) which are embedded into the Go binary via `go:embed` for single-binary deployment.

## Directory Structure

```
web/
├── nuxt.config.ts                         Nuxt configuration
├── package.json                           Dependencies and scripts
├── tsconfig.json                          TypeScript configuration
└── app/
    ├── app.vue                            Root component (SSE connection)
    ├── assets/css/main.css                Tailwind + design system
    ├── middleware/
    │   └── auth.ts                        Route protection middleware
    ├── pages/
    │   ├── index.vue                      Video library grid
    │   ├── login.vue                      Login form
    │   ├── settings.vue                   Settings hub (tabbed)
    │   └── watch/[id].vue                 Video player + details
    ├── components/
    │   ├── AppHeader.vue                  Navigation bar
    │   ├── VideoCard.vue                  Thumbnail card for grid
    │   ├── VideoGrid.vue                  Responsive grid wrapper
    │   ├── VideoPlayer.vue                video.js player with sprites
    │   ├── VideoUpload.vue                Drag-and-drop upload form
    │   ├── VideoMetadata.vue              Desktop sidebar with video info
    │   ├── Pagination.vue                 Page navigation
    │   ├── UploadIndicator.vue            Active upload progress bars
    │   ├── watch/
    │   │   ├── DetailTabs.vue             Tab switcher (Jobs/Thumbnail)
    │   │   ├── Jobs.vue                   Per-video job history
    │   │   └── Thumbnail.vue             Thumbnail extraction/upload UI
    │   ├── settings/
    │   │   ├── Account.vue                Username/password forms
    │   │   ├── Player.vue                 Autoplay, volume, loop
    │   │   ├── App.vue                    Videos per page, sort order
    │   │   ├── Users.vue                  Admin user management
    │   │   ├── Jobs.vue                   Admin jobs panel (4 sub-tabs)
    │   │   ├── UserCreateModal.vue        Create user modal
    │   │   ├── UserEditRoleModal.vue      Change role modal
    │   │   ├── UserResetPasswordModal.vue Reset password modal
    │   │   ├── UserDeleteModal.vue        Delete confirmation modal
    │   │   └── jobs/
    │   │       ├── Workers.vue            Worker pool size config
    │   │       ├── Processing.vue         Quality/concurrency settings
    │   │       └── Triggers.vue           Phase trigger configuration
    │   └── ui/
    │       ├── ErrorAlert.vue             Reusable error display
    │       └── LoadingSpinner.vue         Reusable loading indicator
    ├── stores/
    │   ├── auth.ts                        Auth state + token persistence
    │   ├── videos.ts                      Video list + pagination
    │   ├── upload.ts                      Upload queue + progress
    │   └── settings.ts                    User preferences
    ├── composables/
    │   ├── useApi.ts                      Centralized API layer
    │   ├── useSSE.ts                      SSE connection management
    │   ├── useFormatter.ts                Duration, size, bitrate formatting
    │   ├── useSettingsMessage.ts           Form message/error state
    │   ├── useThumbnailPreview.ts         Sprite hover preview
    │   └── useVttParser.ts                VTT cue file parser
    ├── types/
    │   ├── video.ts                       Video, VideoListResponse
    │   ├── auth.ts                        User, AuthResponse, LoginRequest
    │   ├── settings.ts                    UserSettings, SortOrder
    │   ├── admin.ts                       AdminUser, RoleResponse, PermissionResponse
    │   └── jobs.ts                        JobHistory, PoolConfig, ProcessingConfig, TriggerConfig
    └── utils/
        └── video.ts                       isVideoProcessing(), hasVideoError()
```

## Pages & Routing

### Route Map

| Path | Page | Auth | Description |
|------|------|------|-------------|
| `/` | `index.vue` | Required | Video library grid with upload and pagination |
| `/login` | `login.vue` | Public | Username/password login form |
| `/settings` | `settings.vue` | Required | Tabbed settings (account, player, app, users, jobs) |
| `/watch/:id` | `watch/[id].vue` | Required | Video player with metadata sidebar and detail tabs |

### Auth Middleware

Route protection is handled by `middleware/auth.ts`, applied via `definePageMeta({ middleware: ['auth'] })`:

```
Request to protected route
  │
  ├─ No token? ──────────────► Redirect to /login?redirect=<original-path>
  │
  ├─ Token but no user? ────► Fetch /api/v1/auth/me
  │   ├─ Success ──────────► Load settings, continue to page
  │   └─ 401 ──────────────► Clear token, redirect to /login
  │
  └─ User visits /login ───► Redirect to / (or ?redirect= target)
```

On first page load after a browser restart, the middleware rehydrates the user from the API using the persisted session token.

## Component Architecture

### Naming & Auto-Import

Components are auto-imported by Nuxt based on directory structure:

| File Path | Component Name |
|-----------|---------------|
| `components/VideoCard.vue` | `<VideoCard />` |
| `components/settings/Account.vue` | `<SettingsAccount />` |
| `components/settings/jobs/Workers.vue` | `<SettingsJobsWorkers />` |
| `components/watch/Jobs.vue` | `<WatchJobs />` |
| `components/ui/ErrorAlert.vue` | `<UiErrorAlert />` |

### Component Patterns

**Self-Sufficient Sub-Components:**

Sub-components manage their own state via stores and composables. Parent pages are thin orchestrators (tab selection, layout) that pass no props to tab-level children.

```
settings.vue (parent)
  │ Only manages: activeTab, conditional rendering
  │
  ├─ <SettingsAccount />    ← owns form state, calls useApi()
  ├─ <SettingsPlayer />     ← owns form state, reads useSettingsStore()
  ├─ <SettingsApp />        ← owns form state, reads useSettingsStore()
  ├─ <SettingsUsers />      ← owns user list, modals, useApi()
  └─ <SettingsJobs />       ← owns sub-tabs, delegates to jobs/* children
```

**Modal Pattern:**

All modals follow a consistent contract:

```typescript
// Props
defineProps<{
    visible: boolean;
    user?: AdminUser;   // Entity data (optional)
}>();

// Emits
defineEmits<{
    close: [];
    created: [];         // Success event (varies by modal)
}>();
```

- Rendered with `<Teleport to="body">`
- Backdrop closes on `@click.self="handleClose"`
- Self-contained form state (not passed in)
- Display errors internally
- Reset form after successful submission

**Data Loading with `v-if` Tabs:**

Components rendered with `v-if` mount fresh each time the tab activates. Data loading happens in `onMounted()` with no need for watchers on the parent:

```vue
<!-- Parent: settings.vue -->
<SettingsUsers v-if="activeTab === 'users'" />

<!-- Child: settings/Users.vue -->
<script setup lang="ts">
onMounted(async () => {
    await loadUsers();    // Fresh load every time tab is shown
    await loadRoles();
});
</script>
```

### Provider Pattern (Watch Page)

The watch page uses Vue's `provide/inject` to pass context to deeply nested children without prop drilling:

```typescript
// watch/[id].vue
const playerTime = ref(0);
provide('getPlayerTime', () => playerTime.value);
provide('watchVideo', video);

// components/watch/Thumbnail.vue
const getPlayerTime = inject<() => number>('getPlayerTime');
const video = inject<Ref<Video>>('watchVideo');
```

## State Management (Pinia)

### Store Overview

```
┌─────────────────┐    ┌──────────────────┐
│   useAuthStore  │    │ useSettingsStore │
│                 │    │                  │
│ user, token     │    │ settings         │
│ login()         │    │ loadSettings()   │
│ logout()        │    │ updatePlayer()   │
│ fetchCurrentUser│    │ updateApp()      │
└────────┬────────┘    └────────┬─────────┘
         │                      │
         │    ┌─────────────────┼
         │    │                 │         
    ┌────▼────▼────┐    ┌───────▼────────┐
    │useUploadStore│    │ useVideoStore  │
    │              │    │                │
    │ uploads      │    │ videos, total  │
    │ addUpload()  │    │ loadVideos()   │
    │ processQueue │    │ updateFields() │
    └──────────────┘    │ prependVideo() │
                        └────────────────┘
```

### Store Dependencies

- `useVideoStore` → reads `useSettingsStore().videosPerPage` to sync limit
- `useUploadStore` → calls `useVideoStore().prependVideo()` on upload completion
- `useUploadStore` → reads `useAuthStore().token` for upload auth headers

### Persistence

Stores use `@pinia-plugin-persistedstate/nuxt` with **sessionStorage** (cleared on browser close):

| Store | Persisted Fields |
|-------|-----------------|
| `auth` | `user`, `token` |
| `settings` | `settings` |
| `videos` | Not persisted |
| `upload` | Not persisted |

### Video Store: Real-Time Updates

The video store exposes `updateVideoFields(videoId, fields)` for reactive SSE updates:

```typescript
// Called by useSSE composable on each event
videoStore.updateVideoFields(event.video_id, {
    thumbnail_path: event.data?.thumbnail_path,
});
```

This patches the video object in-place, triggering Vue reactivity for any component displaying that video.

## Composables

### useApi — API Communication Layer

Centralized HTTP client wrapping the native `fetch` API:

```typescript
const { fetchVideos, uploadVideo, fetchSettings, ... } = useApi();
```

**Key behaviors:**
- Injects `Authorization: Bearer <token>` from auth store
- Returns parsed JSON on success
- On 401: calls `authStore.logout()` and throws
- On other errors: parses `{ error: string }` body and throws with the message
- File uploads omit `Content-Type` header (let browser set multipart boundary)

**Endpoint groups:**

| Group | Endpoints |
|-------|-----------|
| Auth | `login`, `fetchCurrentUser`, `logout` (handled directly in auth store) |
| Videos | `uploadVideo`, `fetchVideos`, `fetchVideo`, `extractThumbnail`, `uploadThumbnail` |
| Settings | `fetchSettings`, `updatePlayerSettings`, `updateAppSettings`, `changePassword`, `changeUsername` |
| Admin Users | `fetchAdminUsers`, `createUser`, `updateUserRole`, `resetUserPassword`, `deleteUser` |
| Admin RBAC | `fetchRoles`, `fetchPermissions`, `syncRolePermissions` |
| Admin Jobs | `fetchJobs`, `fetchPoolConfig`, `updatePoolConfig`, `fetchProcessingConfig`, `updateProcessingConfig`, `fetchTriggerConfig`, `updateTriggerConfig`, `triggerVideoPhase` |

### useSSE — Real-Time Event Streaming

Manages an EventSource connection to receive processing updates:

```
App Mount (authenticated)
  │
  └─ connect()
      │
      ├─ EventSource(/api/v1/events?token=<token>)
      │
      ├─ on video:metadata_complete ──► update duration, width, height
      ├─ on video:thumbnail_complete ─► update thumbnail_path
      ├─ on video:sprites_complete ───► update vtt_path, sprite_sheet_path
      ├─ on video:completed ──────────► set processing_status = 'completed'
      ├─ on video:failed ─────────────► set processing_status = 'failed'
      │
      └─ on error ──► close, scheduleReconnect()
                        │
                        ├─ Wait reconnectDelay (1s → 2s → 4s → ... → 30s max)
                        ├─ Reload current video page
                        └─ Reconnect
```

**Key behaviors:**
- Token passed as query parameter (EventSource API doesn't support custom headers)
- Reconnect delay resets to 1s on successful connection (`onopen`)
- On reconnect, refreshes the current video list page to catch missed updates
- Disconnect clears all timers and closes the connection

### useFormatter — Display Formatting

Pure utility functions with no state:

| Function | Input | Output Example |
|----------|-------|---------------|
| `formatDuration(seconds)` | `3661` | `1:01:01` |
| `formatSize(bytes)` | `1073741824` | `1.00 GB` |
| `formatBitRate(bps)` | `5000000` | `5.0 Mbps` |
| `formatFrameRate(fps)` | `23.976` | `23.976 fps` |

### useSettingsMessage — Form State

Provides consistent message/error handling for settings forms:

```typescript
const { message, error, clearMessages, setError } = useSettingsMessage();

// On success:
clearMessages();
message.value = 'Settings saved';

// On failure:
setError(e, 'Failed to save settings');
```

### useThumbnailPreview — Sprite Sheet Hover

Calculates sprite coordinates for seek bar thumbnail preview based on mouse position, sprite grid dimensions, and VTT cue data.

### useVttParser — VTT Cue Parsing

Parses WebVTT files to extract timestamp ranges and corresponding sprite sheet coordinates (`x`, `y`, `w`, `h`) for thumbnail preview mapping.

## Authentication Flow

```
┌─────────────────────────────────────────────────────────┐
│                    Session Lifecycle                    │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Login Page                                             │
│  ┌─────────────────────────────────────┐                │
│  │ POST /api/v1/auth/login             │                │
│  │ Response: { token, user }           │                │
│  │ Store token + user in sessionStorage│                │
│  └──────────────────┬──────────────────┘                │
│                     │                                   │
│                     ▼                                   │
│  Authenticated Session                                  │
│  ┌─────────────────────────────────────┐                │
│  │ Token in sessionStorage             │                │
│  │ Auth header: Bearer <token>         │                │
│  │ SSE connection: ?token=<token>      │                │
│  └──────────────────┬──────────────────┘                │
│                     │                                   │
│            ┌────────┴────────┐                          │
│            ▼                 ▼                          │
│  401 Response           Logout Click                    │
│  ┌──────────────┐   ┌──────────────────┐                │
│  │ Clear token  │   │ POST /auth/logout│                │
│  │ Clear user   │   │ Clear token      │                │
│  │ Redirect     │   │ Clear user       │                │
│  │ to /login    │   │ Redirect /login  │                │
│  └──────────────┘   └──────────────────┘                │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Role-Based UI Visibility:**

The frontend uses the `user.role` field from the auth store to conditionally render admin-only tabs and features. Actual authorization is enforced by the backend API.

```typescript
const availableTabs = computed(() => {
    const tabs = ['account', 'player', 'app'];
    if (authStore.user?.role === 'admin') {
        tabs.push('users', 'jobs');
    }
    return tabs;
});
```

## Upload System

The upload store implements a concurrent upload queue with progress tracking:

```
addUpload(file, title)
  │
  ├─ Create UploadItem { id, file, title, status: 'queued', progress: 0 }
  │
  └─ processQueue()
      │
      ├─ Count active uploads (status === 'uploading')
      ├─ Slots available = MAX_CONCURRENT (2) - active
      │
      └─ For each available slot:
          └─ startUpload(item)
              │
              ├─ item.status = 'uploading'
              ├─ Create XHR with progress handler
              │   └─ xhr.upload.onprogress → item.progress = %
              │
              ├─ On success:
              │   ├─ item.status = 'completed'
              │   ├─ videoStore.prependVideo(response)  (if on page 1)
              │   └─ processQueue()  (start next queued)
              │
              ├─ On 401:
              │   ├─ item.status = 'failed'
              │   └─ authStore.logout()
              │
              └─ On error:
                  ├─ item.status = 'failed'
                  └─ processQueue()
```

**Why XHR instead of fetch:** The upload store uses native `XMLHttpRequest` rather than `fetch` because XHR provides granular upload progress events via `xhr.upload.onprogress`, which `fetch` does not support.

## Video Player

The `VideoPlayer` component integrates video.js with custom features:

- **Stream URL:** Constructed as `/api/v1/videos/:id/stream`
- **Poster:** Uses thumbnail URL `/thumbnails/:id?size=lg`
- **Sprite Previews:** Parses VTT cues to map seek bar hover position to sprite sheet coordinates
- **Custom Elements:** Vue compiler configured to treat `media-*`, `videojs-video`, `media-theme` as native elements (not Vue components)
- **Player Settings:** Respects `autoplay`, `default_volume`, `loop` from settings store

## Design System

### Color Palette

The design uses CSS custom properties defined in Tailwind's `@theme` block:

| Token | Value | Usage |
|-------|-------|-------|
| `--color-void` | `#050505` | Page background (deepest black) |
| `--color-surface` | `#0a0a0a` | Slightly elevated areas |
| `--color-panel` | `#0f0f0f` | Modal/card backgrounds |
| `--color-elevated` | `#141414` | Highest elevation layer |
| `--color-lava` | `#ff4d4d` | Primary accent (buttons, active tabs, links) |
| `--color-lava-glow` | `#ff6b6b` | Hover state for lava elements |
| `--color-ember` | `#ff2d2d` | Intense red (errors, destructive actions) |
| `--color-emerald` | `#10b981` | Success states, active toggles |
| `--color-muted` | `rgba(255,255,255,0.6)` | Secondary text |
| `--color-dim` | `rgba(255,255,255,0.35)` | Disabled/tertiary text |
| `--color-border` | `rgba(255,255,255,0.08)` | Subtle borders |
| `--color-border-hover` | `rgba(255,255,255,0.15)` | Interactive borders |

### Typography

| Token | Value | Usage |
|-------|-------|-------|
| `--font-sans` | `'Outfit', system-ui, sans-serif` | Primary text |
| `--font-mono` | `'JetBrains Mono', monospace` | Technical data, metadata |

Body font size is `13px` with `line-height: 1.5`.

### Custom Utilities

| Utility | Effect |
|---------|--------|
| `glass-panel` | `backdrop-blur(20px)` + semi-transparent bg + subtle border, `border-radius: 12px` |
| `glass-card` | Lighter glass effect, `border-radius: 10px` |
| `glow-lava` | Red shadow glow at 15% opacity |
| `glow-lava-strong` | Red shadow glow at 30% opacity |
| `cosmic-bg` | Subtle scattered dot particles via radial gradients |
| `animate-spin-slow` | 3s rotation |
| `animate-pulse-glow` | 2s opacity pulse (0.4 → 1) |
| `animate-float` | 6s vertical float (-6px) |

### Depth Model

The UI uses borders and backdrop blur to create depth rather than shadows:

```
Layer 0: --color-void (#050505)        Page background
Layer 1: --color-surface (#0a0a0a)     Cards, sections
Layer 2: --color-panel (#0f0f0f)       Modals, floating panels
Layer 3: --color-elevated (#141414)    Dropdowns, tooltips
```

Panels are defined by `1px solid rgba(255,255,255,0.08)` borders. Hover states lighten the border to `0.15` opacity.

### Responsive Breakpoints

| Context | Mobile | sm (640px) | lg (1024px) | xl (1280px) | 2xl (1536px) |
|---------|--------|------------|-------------|-------------|-------------|
| Video grid columns | 2 | 3 | 4 | 5 | 6 |
| Video sidebar | Hidden | Hidden | Hidden | Visible (280px) | Visible |
| Settings width | Full | Full | max-w-2xl | max-w-2xl | max-w-2xl |

## TypeScript Types

Type definitions live in `app/types/` and are imported with `import type`:

### Video

```typescript
interface Video {
    id: number;
    title: string;
    original_filename: string;
    size: number;
    view_count: number;
    created_at: string;
    duration: number;
    width?: number;
    height?: number;
    thumbnail_path?: string;
    sprite_sheet_path?: string;
    vtt_path?: string;
    processing_status?: string;
    processing_error?: string;
    frame_rate?: number;
    bit_rate?: number;
    video_codec?: string;
    audio_codec?: string;
    // ... additional metadata fields
}

interface VideoListResponse {
    data: Video[];
    total: number;
    page: number;
    limit: number;
}
```

### Auth

```typescript
interface User {
    id: number;
    username: string;
    role: 'admin' | 'user';
}

interface AuthResponse {
    token: string;
    user: User;
}
```

### Jobs

```typescript
interface JobHistory {
    id: number;
    job_id: string;
    video_id: number;
    video_title: string;
    phase: 'metadata' | 'thumbnail' | 'sprites';
    status: 'running' | 'completed' | 'failed';
    error_message?: string;
    started_at: string;
    completed_at?: string;
}

interface PoolConfig {
    metadata_workers: number;
    thumbnail_workers: number;
    sprites_workers: number;
}

interface TriggerConfig {
    id: number;
    phase: string;
    trigger_type: 'on_import' | 'after_job' | 'manual' | 'scheduled';
    after_phase?: string;
    cron_expression?: string;
}
```

## Development Configuration

### Nuxt Config

Key settings in `nuxt.config.ts`:

- **SSR:** Disabled (`ssr: false`) — the app is a client-side SPA
- **Modules:** `@nuxt/eslint`, `@pinia/nuxt`, `@nuxt/icon`, `@pinia-plugin-persistedstate/nuxt`
- **Custom Elements:** `media-*`, `videojs-video`, `media-theme` treated as native elements
- **Pinia Stores:** Auto-discovered from `./stores/**`
- **Build Output:** `web/dist/` (embedded into Go binary)

### Vite Dev Server Proxy

In development, the Nuxt dev server (`:3000`) proxies backend routes to the Go server (`:8080`):

| Path | Target |
|------|--------|
| `/api/*` | `http://localhost:8080` |
| `/thumbnails/*` | `http://localhost:8080` |
| `/sprites/*` | `http://localhost:8080` |
| `/vtt/*` | `http://localhost:8080` |

### Auto-Imports

Nuxt auto-imports the following — no explicit `import` statements needed:

| Category | Examples |
|----------|---------|
| Vue APIs | `ref`, `computed`, `watch`, `onMounted`, `provide`, `inject` |
| Nuxt utilities | `navigateTo`, `definePageMeta`, `defineNuxtRouteMiddleware` |
| Stores | `useAuthStore()`, `useVideoStore()`, `useUploadStore()`, `useSettingsStore()` |
| Composables | `useApi()`, `useSSE()`, `useFormatter()`, `useSettingsMessage()` |
| Components | All components from `app/components/` tree |

Only TypeScript types require explicit imports: `import type { Video } from '~/types/video'`
