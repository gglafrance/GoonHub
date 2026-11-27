# Header layout (per account)

Each user can rearrange the two header bars: the **top bar** (logo row) and the **nav row**
(links row). Items can be reordered, moved to another zone or bar, grouped into folders
(dropdown buttons) or hidden. The layout is stored per account on the server, so it follows
the user on every device. Editor: Settings > Navigation.

## Model

- **Registry** (fixed in code): every header item, with its id, label, icon, the permission it
  needs, the bars it can go in, whether it can be hidden or put in a folder, and its default
  place. Frontend: `web/app/utils/navRegistry.ts`. Server: `internal/core/nav_layout_catalog.go`
  (same ids, permissions, bars and flags; `web/tests/navLayout.test.ts` reads the Go file and
  fails when they differ).
- **Layout** (per user, JSON): for each bar (`top`, `nav`) three zones (`left`, `center`,
  `right`), each an ordered list of entries: an item id or a folder reference `folder:<id>`.
  Plus the user's folders (`{id, name, icon, items}`) and the hidden items.

```json
{
  "version": 1,
  "bars": {
    "top": { "left": ["logo"], "center": [], "right": ["command_palette", "profile", "..."] },
    "nav": { "left": ["search", "markers", "...", "folder:manage"], "center": [], "right": [] }
  },
  "folders": [{ "id": "manage", "name": "Manage", "icon": "heroicons:wrench-screwdriver",
                "items": ["triage", "explorer", "duplicates"] }],
  "hidden": []
}
```

- **Default layout** = the header as it was before: logo on the left of the top bar, the
  search field and account icons on its right, the user pages in the nav row, then the
  **Manage** folder (Triage, Explorer, Duplicates). The former Manage menu is that folder: it
  can be renamed, given another icon, moved or dissolved like any folder.
- A user without a stored layout follows the default layout (also its later changes).

## Items

| Id | Bars | Folder | Required | Permission |
|---|---|---|---|---|
| `logo` | top | no | yes | |
| `command_palette` (search field) | both | yes (opens the palette) | | |
| `profile` | both | yes (link) | | |
| `shortcuts` | both | yes | | |
| `safe_mode` | both | yes (toggle) | | |
| `preview_sound` | both | yes (toggle) | | |
| `job_status` | both | no (popover) | | `jobs:view_status` |
| `presence` | both | no (popover) | | |
| `messages` | both | yes (link to /messages) | | |
| `friends` | both | yes | | |
| `settings` | both | yes | yes | |
| `logout` | both | yes | yes | |
| `search`, `markers`, `playlists`, `history`, `actors`, `studios`, `triage` | both | yes | | |
| `together` | both | yes | | `goon2gether:use` |
| `explorer` | both | yes | | `explorer:access` |
| `duplicates` | both | yes | | `duplicates:manage` |

Required items cannot be hidden (nobody can lock themselves out of the settings or the
logout). Items with a popover anchored to their button (job status, presence) stay in a bar.
Each item renders the same way in both bars; in a folder it is a menu row (a link, or an action
for the toggles, the palette, the shortcuts and the logout).

## Resolution

`completeNavLayout` (frontend) builds what is shown from the stored layout (or the default
one): it repairs it like the server does (unknown, duplicated or misplaced entries dropped),
drops the items the user lacks the permission for, then puts every allowed item the layout
does not mention at its default place (its default folder if it still exists, else its
default zone). New items of a later version and items whose permission was granted later
therefore appear without any migration. A folder without a visible item is not shown; an empty
nav row is not shown.

## Permissions

An item the user lacks the permission for is never shown, never offered in the editor, and
never stored: `GET` leaves it out, `PUT` strips it (silently) from the saved layout. A user who
loses a permission just stops seeing the item (no error); it comes back at its default place if
the permission is granted again.

## API

All routes need a signed-in user (no extra permission).

- `GET /api/v1/settings/navigation`: `{layout, customized}`. `customized` is false when the
  user has no stored layout (the default layout is answered). A stored layout is repaired when
  read (entries of an older catalog dropped); forbidden items are left out.
- `PUT /api/v1/settings/navigation`: body `{layout}` (64 KB max). Validated (`400` on the first
  problem): `version` 1; known item ids; each item and each folder placed once; an item in a
  bar it allows; only folderable items in folders, no folder in a folder; required items not
  hidden; at most 12 folders; folder ids `^[a-z0-9][a-z0-9_-]{0,31}$`, unique; names 1 to 32
  characters (trimmed, no control characters); icons `heroicons:<name>` or empty. Items the user
  lacks the permission for are removed. Answers the stored layout, `customized: true`.
- `DELETE /api/v1/settings/navigation`: removes the stored layout, answers the default one.

`PUT` and `DELETE` publish the personal event `settings:nav_layout` (no data,
`EventBus.PublishToUser`): the other tabs and devices of the user reload the layout.

Storage: table `user_nav_layouts` (migration 106, `docs/DATABASE.md`).

## Frontend

- `stores/navLayout.ts`: the layout of the signed-in user, cached per user in localStorage (no
  flicker from the default layout on load), reloaded on `settings:nav_layout` and on SSE
  reconnection.
- `components/AppHeader.vue` resolves the layout; `header/LayoutBar.vue` lays a bar out in its
  three zones, `header/Entry.vue` renders an item, `header/NavFolder.vue` a folder (dropdown:
  click to open, Esc, outside click, Tab out, arrows, Home and End; a dot when an item inside has
  a badge), `header/FolderItem.vue` a menu row.
- `header/LiveSources.vue` starts the live state behind the items once (social summary, messages
  and their toasts, presence, public lobby count, the layout), whatever is shown: badges work
  wherever an item is placed, even in a folder or hidden.
- `useHeaderItems()`: badges, menu row labels (toggles show their state) and actions.
- Settings > Navigation (`settings/Navigation.vue`, `settings/navigation/*`): live preview with
  the real bars, the zones of both bars and the Hidden tray as drop areas (HTML5 drag and drop),
  and a toolbar for the selected entry (move left/right, move to a zone, a folder or Hidden,
  show again; rename, icon and dissolve for a folder) for keyboards and touch screens. New
  folder, Reset to default (saving it deletes the stored layout), Save and Discard.

## Mobile

On small screens the zones collapse: in the top bar the left and center zones stay together at
the start and the right zone at the end; the nav row lays every entry out in one row (spread out
and scrolling horizontally when it does not fit, as before). Labels are hidden as before (icons
only) and folders stay dropdowns.
