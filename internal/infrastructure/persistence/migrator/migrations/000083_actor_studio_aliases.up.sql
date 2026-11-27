-- Aliases for studios, the same way as actors (migration 041): other names under which
-- a studio is known. Resolution by name (import clients, PornDB matching, the resolve
-- endpoint) also matches them, case-insensitively.
-- (IF NOT EXISTS: a database that already has the column keeps its aliases, and they are
-- used by the merge below.)
ALTER TABLE studios ADD COLUMN IF NOT EXISTS aliases TEXT[] NOT NULL DEFAULT '{}';
CREATE INDEX IF NOT EXISTS idx_studios_aliases ON studios USING GIN (aliases);

-- actors.aliases was nullable: same shape as studios.aliases
UPDATE actors SET aliases = '{}' WHERE aliases IS NULL;
ALTER TABLE actors ALTER COLUMN aliases SET NOT NULL;

-- Case-insensitive lookups by exact name (resolve, duplicate check)
CREATE INDEX idx_actors_name_lower ON actors (lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX idx_studios_name_lower ON studios (lower(name)) WHERE deleted_at IS NULL;

-- Merging an actor or a studio into another one: admin and moderator
INSERT INTO permissions (name, description, created_at) VALUES
    ('actors:merge', 'Merge an actor into another one', NOW()),
    ('studios:merge', 'Merge a studio into another one', NOW())
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name IN ('admin', 'moderator') AND p.name IN ('actors:merge', 'studios:merge')
ON CONFLICT DO NOTHING;

-- =====================================================================================
-- Merge the actors (and studios) that were split into one entity per alias.
--
-- Before aliases were resolved, a name could be created as its own actor although it is
-- an alias of another one. Such an actor B is merged into actor A when B's name (trimmed,
-- case-insensitive) equals an alias of exactly ONE other non-deleted actor A. The plan is
-- computed once, on the data as it is before any merge:
--   * ambiguous: B's name is an alias of several actors -> B is left alone;
--   * mutual aliases (A's name is an alias of B and B's name an alias of A): the oldest
--     (lowest id) is kept, the other one is merged into it;
--   * chains (B -> A and A -> C): every actor of the chain is merged directly into the
--     final target C (the end of the chain);
--   * loops of 3 or more actors (A -> B -> C -> A), and the actors whose chain leads into
--     such a loop, have no final target: they are left alone.
-- Each merge does what the merge tool (ActorRepository.Merge) does: scene credits move to
-- the target (no duplicates), ratings and likes move (a user's own rating/like of the target
-- is kept; with several sources, the first merged source, lowest id, wins), homepage
-- sections referencing the source's UUID and saved search / saved marker filters
-- referencing its name point to the target, the source's name and aliases become aliases of
-- the target (deduplicated case-insensitively, without the target's own name), the target
-- keeps its photo unless it has none, and the source is soft-deleted. Studios: same rule,
-- plus scenes (studio_id and the denormalized scenes.studio) and child/network studios.
-- Merges are applied in source id order. The down migration cannot undo them.
-- =====================================================================================

DROP TABLE IF EXISTS pg_temp.alias_merge_actor_plan;
CREATE TEMP TABLE alias_merge_actor_plan AS
WITH RECURSIVE
live AS (SELECT id, name, aliases FROM actors WHERE deleted_at IS NULL),
owners AS (
    -- (src, dst): an alias of dst is src's name
    SELECT DISTINCT s.id AS src, d.id AS dst
    FROM live s
    JOIN live d ON d.id <> s.id
    JOIN LATERAL unnest(d.aliases) AS al(v) ON lower(btrim(al.v)) = lower(btrim(s.name))
    WHERE btrim(s.name) <> ''
),
single AS (
    SELECT src, min(dst) AS dst FROM owners GROUP BY src HAVING count(*) = 1
),
edges AS (
    -- mutual aliases: drop the edge of the oldest, it is the one kept
    SELECT e.src, e.dst FROM single e
    WHERE NOT (e.src < e.dst AND EXISTS (SELECT 1 FROM single r WHERE r.src = e.dst AND r.dst = e.src))
),
walk(src, cur, path, looped) AS (
    SELECT src, dst, ARRAY[src, dst], false FROM edges
    UNION ALL
    SELECT w.src, e.dst, w.path || e.dst, e.dst = ANY(w.path)
    FROM walk w JOIN edges e ON e.src = w.cur
    WHERE NOT w.looped
)
SELECT w.src, w.cur AS dst
FROM walk w
WHERE NOT w.looped
  AND NOT EXISTS (SELECT 1 FROM edges e WHERE e.src = w.cur)
  AND NOT EXISTS (SELECT 1 FROM walk l WHERE l.src = w.src AND l.looped);

DO $$
DECLARE
    pair  RECORD;
    s     actors%ROWTYPE;
    d     actors%ROWTYPE;
    moved BIGINT[];
BEGIN
    FOR pair IN SELECT src, dst FROM alias_merge_actor_plan ORDER BY src LOOP
        SELECT * INTO s FROM actors WHERE id = pair.src;
        SELECT * INTO d FROM actors WHERE id = pair.dst;

        -- scene credits (no duplicates), then the denormalized scenes.actors
        SELECT array_agg(scene_id) INTO moved FROM scene_actors WHERE actor_id = s.id;
        INSERT INTO scene_actors (scene_id, actor_id)
            SELECT scene_id, d.id FROM scene_actors WHERE actor_id = s.id
            ON CONFLICT DO NOTHING;
        DELETE FROM scene_actors WHERE actor_id = s.id;

        -- ratings and likes: the target's own ones win
        UPDATE user_actor_ratings r SET actor_id = d.id, updated_at = NOW()
            WHERE r.actor_id = s.id
            AND NOT EXISTS (SELECT 1 FROM user_actor_ratings t WHERE t.user_id = r.user_id AND t.actor_id = d.id);
        DELETE FROM user_actor_ratings WHERE actor_id = s.id;
        UPDATE user_actor_likes l SET actor_id = d.id
            WHERE l.actor_id = s.id
            AND NOT EXISTS (SELECT 1 FROM user_actor_likes t WHERE t.user_id = l.user_id AND t.actor_id = d.id);
        DELETE FROM user_actor_likes WHERE actor_id = s.id;

        -- homepage sections referencing the source by UUID
        UPDATE user_settings
            SET homepage_config = replace(homepage_config::text, s.uuid::text, d.uuid::text)::jsonb
            WHERE strpos(homepage_config::text, s.uuid::text) > 0;

        -- saved filters referencing the source by name (exact), without duplicates
        IF s.name <> d.name THEN
            UPDATE saved_searches SET updated_at = NOW(), filters = jsonb_set(filters, '{selected_actors}', (
                SELECT COALESCE(jsonb_agg(v ORDER BY ord), '[]'::jsonb) FROM (
                    SELECT DISTINCT ON (v) v, ord FROM (
                        SELECT CASE WHEN e = s.name THEN d.name ELSE e END AS v, ord
                        FROM jsonb_array_elements_text(filters->'selected_actors') WITH ORDINALITY AS x(e, ord)
                    ) renamed ORDER BY v, ord
                ) dedup))
                WHERE jsonb_typeof(filters->'selected_actors') = 'array'
                AND jsonb_exists(filters->'selected_actors', s.name);
            UPDATE saved_marker_filters SET updated_at = NOW(), filters = jsonb_set(filters, '{actors}', (
                SELECT COALESCE(jsonb_agg(v ORDER BY ord), '[]'::jsonb) FROM (
                    SELECT DISTINCT ON (v) v, ord FROM (
                        SELECT CASE WHEN e = s.name THEN d.name ELSE e END AS v, ord
                        FROM jsonb_array_elements_text(filters->'actors') WITH ORDINALITY AS x(e, ord)
                    ) renamed ORDER BY v, ord
                ) dedup))
                WHERE jsonb_typeof(filters->'actors') = 'array'
                AND jsonb_exists(filters->'actors', s.name);
        END IF;

        -- the target: aliases (its own, then the source's name and aliases), photo if missing
        UPDATE actors t SET
            aliases = COALESCE((
                SELECT array_agg(v ORDER BY ord) FROM (
                    SELECT DISTINCT ON (lower(v)) v, ord FROM (
                        SELECT btrim(raw) AS v, ord
                        FROM unnest(t.aliases || ARRAY[s.name] || s.aliases) WITH ORDINALITY AS u(raw, ord)
                    ) trimmed
                    WHERE v <> '' AND lower(v) <> lower(btrim(t.name))
                    ORDER BY lower(v), ord
                ) dedup), '{}'),
            image_url = CASE WHEN COALESCE(t.image_url, '') = '' THEN s.image_url ELSE t.image_url END,
            updated_at = NOW()
            WHERE t.id = d.id;

        UPDATE actors SET deleted_at = NOW() WHERE id = s.id;

        IF moved IS NOT NULL THEN
            UPDATE scenes sc SET actors = COALESCE((
                    SELECT array_agg(a.name ORDER BY a.name) FROM scene_actors sa
                    JOIN actors a ON a.id = sa.actor_id AND a.deleted_at IS NULL
                    WHERE sa.scene_id = sc.id), '{}')
                WHERE sc.id = ANY(moved);
        END IF;

        RAISE NOTICE 'migration 083: actor % (%) merged into actor % (%)', s.id, s.name, d.id, d.name;
    END LOOP;
END
$$;

DROP TABLE IF EXISTS pg_temp.alias_merge_studio_plan;
CREATE TEMP TABLE alias_merge_studio_plan AS
WITH RECURSIVE
live AS (SELECT id, name, aliases FROM studios WHERE deleted_at IS NULL),
owners AS (
    SELECT DISTINCT s.id AS src, d.id AS dst
    FROM live s
    JOIN live d ON d.id <> s.id
    JOIN LATERAL unnest(d.aliases) AS al(v) ON lower(btrim(al.v)) = lower(btrim(s.name))
    WHERE btrim(s.name) <> ''
),
single AS (
    SELECT src, min(dst) AS dst FROM owners GROUP BY src HAVING count(*) = 1
),
edges AS (
    SELECT e.src, e.dst FROM single e
    WHERE NOT (e.src < e.dst AND EXISTS (SELECT 1 FROM single r WHERE r.src = e.dst AND r.dst = e.src))
),
walk(src, cur, path, looped) AS (
    SELECT src, dst, ARRAY[src, dst], false FROM edges
    UNION ALL
    SELECT w.src, e.dst, w.path || e.dst, e.dst = ANY(w.path)
    FROM walk w JOIN edges e ON e.src = w.cur
    WHERE NOT w.looped
)
SELECT w.src, w.cur AS dst
FROM walk w
WHERE NOT w.looped
  AND NOT EXISTS (SELECT 1 FROM edges e WHERE e.src = w.cur)
  AND NOT EXISTS (SELECT 1 FROM walk l WHERE l.src = w.src AND l.looped);

DO $$
DECLARE
    pair RECORD;
    s    studios%ROWTYPE;
    d    studios%ROWTYPE;
BEGIN
    FOR pair IN SELECT src, dst FROM alias_merge_studio_plan ORDER BY src LOOP
        SELECT * INTO s FROM studios WHERE id = pair.src;
        SELECT * INTO d FROM studios WHERE id = pair.dst;

        -- scenes, with the denormalized scenes.studio
        UPDATE scenes SET studio_id = d.id, studio = d.name WHERE studio_id = s.id;

        -- the target must not end up its own parent/network
        UPDATE studios SET
            parent_id = CASE WHEN parent_id = s.id
                THEN (CASE WHEN s.parent_id = d.id THEN NULL ELSE s.parent_id END)
                ELSE parent_id END,
            network_id = CASE WHEN network_id = s.id
                THEN (CASE WHEN s.network_id = d.id THEN NULL ELSE s.network_id END)
                ELSE network_id END
            WHERE id = d.id;
        -- child and network studios
        UPDATE studios SET parent_id = d.id WHERE parent_id = s.id AND id <> d.id;
        UPDATE studios SET network_id = d.id WHERE network_id = s.id AND id <> d.id;

        -- ratings and likes: the target's own ones win
        UPDATE user_studio_ratings r SET studio_id = d.id, updated_at = NOW()
            WHERE r.studio_id = s.id
            AND NOT EXISTS (SELECT 1 FROM user_studio_ratings t WHERE t.user_id = r.user_id AND t.studio_id = d.id);
        DELETE FROM user_studio_ratings WHERE studio_id = s.id;
        UPDATE user_studio_likes l SET studio_id = d.id
            WHERE l.studio_id = s.id
            AND NOT EXISTS (SELECT 1 FROM user_studio_likes t WHERE t.user_id = l.user_id AND t.studio_id = d.id);
        DELETE FROM user_studio_likes WHERE studio_id = s.id;

        UPDATE user_settings
            SET homepage_config = replace(homepage_config::text, s.uuid::text, d.uuid::text)::jsonb
            WHERE strpos(homepage_config::text, s.uuid::text) > 0;

        IF s.name <> d.name THEN
            UPDATE saved_searches SET updated_at = NOW(), filters = jsonb_set(filters, '{studio}', to_jsonb(d.name::text))
                WHERE filters->>'studio' = s.name;
            UPDATE saved_marker_filters SET updated_at = NOW(), filters = jsonb_set(filters, '{studio}', to_jsonb(d.name::text))
                WHERE filters->>'studio' = s.name;
        END IF;

        -- the target: aliases, then logo, favicon and poster when missing
        UPDATE studios t SET
            aliases = COALESCE((
                SELECT array_agg(v ORDER BY ord) FROM (
                    SELECT DISTINCT ON (lower(v)) v, ord FROM (
                        SELECT btrim(raw) AS v, ord
                        FROM unnest(t.aliases || ARRAY[s.name] || s.aliases) WITH ORDINALITY AS u(raw, ord)
                    ) trimmed
                    WHERE v <> '' AND lower(v) <> lower(btrim(t.name))
                    ORDER BY lower(v), ord
                ) dedup), '{}'),
            logo = CASE WHEN COALESCE(t.logo, '') = '' THEN s.logo ELSE t.logo END,
            favicon = CASE WHEN COALESCE(t.favicon, '') = '' THEN s.favicon ELSE t.favicon END,
            poster = CASE WHEN COALESCE(t.poster, '') = '' THEN s.poster ELSE t.poster END,
            updated_at = NOW()
            WHERE t.id = d.id;

        UPDATE studios SET deleted_at = NOW() WHERE id = s.id;

        RAISE NOTICE 'migration 083: studio % (%) merged into studio % (%)', s.id, s.name, d.id, d.name;
    END LOOP;
END
$$;

DROP TABLE pg_temp.alias_merge_actor_plan;
DROP TABLE pg_temp.alias_merge_studio_plan;
