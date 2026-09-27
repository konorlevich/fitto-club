-- +goose Up
-- Portable DDL: application-generated TEXT ids, INTEGER booleans, TEXT
-- timestamps (RFC 3339) and dates. No AUTOINCREMENT/SERIAL, so the same file
-- runs on SQLite and Postgres (checklist §13: dual-dialect migrations).
-- Soft-delete everywhere editors can delete: deleted_at <> '' hides a row.

CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE memberships (
    id         TEXT PRIMARY KEY,
    slug       TEXT NOT NULL UNIQUE,
    sort       INTEGER NOT NULL DEFAULT 0,
    visible    INTEGER NOT NULL DEFAULT 1,
    name_en    TEXT NOT NULL DEFAULT '',
    name_ru    TEXT NOT NULL DEFAULT '',
    name_ka    TEXT NOT NULL DEFAULT '',
    desc_en    TEXT NOT NULL DEFAULT '',
    desc_ru    TEXT NOT NULL DEFAULT '',
    desc_ka    TEXT NOT NULL DEFAULT '',
    gift_pt    INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT '',
    deleted_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE membership_terms (
    membership_id TEXT NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    months        INTEGER NOT NULL,
    price         INTEGER NOT NULL DEFAULT 0,
    freeze_weeks  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (membership_id, months)
);

CREATE TABLE tags (
    slug    TEXT PRIMARY KEY,
    sort    INTEGER NOT NULL DEFAULT 0,
    name_en TEXT NOT NULL DEFAULT '',
    name_ru TEXT NOT NULL DEFAULT '',
    name_ka TEXT NOT NULL DEFAULT ''
);

CREATE TABLE coaches (
    id          TEXT PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    sort        INTEGER NOT NULL DEFAULT 0,
    published   INTEGER NOT NULL DEFAULT 0,
    photo       INTEGER NOT NULL DEFAULT 0,
    instagram   TEXT NOT NULL DEFAULT '',
    name_en     TEXT NOT NULL DEFAULT '',
    name_ru     TEXT NOT NULL DEFAULT '',
    name_ka     TEXT NOT NULL DEFAULT '',
    bio_en      TEXT NOT NULL DEFAULT '',
    bio_ru      TEXT NOT NULL DEFAULT '',
    bio_ka      TEXT NOT NULL DEFAULT '',
    pt_price    INTEGER NOT NULL DEFAULT 0,
    pt_currency TEXT NOT NULL DEFAULT 'GEL',
    pt_from     INTEGER NOT NULL DEFAULT 0,
    updated_at  TEXT NOT NULL DEFAULT '',
    deleted_at  TEXT NOT NULL DEFAULT ''
);

CREATE TABLE coach_tags (
    coach_id TEXT NOT NULL REFERENCES coaches(id) ON DELETE CASCADE,
    tag_slug TEXT NOT NULL REFERENCES tags(slug) ON DELETE CASCADE,
    sort     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (coach_id, tag_slug)
);

CREATE TABLE coach_langs (
    coach_id TEXT NOT NULL REFERENCES coaches(id) ON DELETE CASCADE,
    lang     TEXT NOT NULL,
    sort     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (coach_id, lang)
);

-- A renamed slug keeps answering with a 301 to the current one.
CREATE TABLE slug_history (
    old_slug TEXT PRIMARY KEY,
    coach_id TEXT NOT NULL REFERENCES coaches(id) ON DELETE CASCADE
);

CREATE TABLE reviews (
    id         TEXT PRIMARY KEY,
    author     TEXT NOT NULL DEFAULT '',
    lang       TEXT NOT NULL DEFAULT 'en',
    body       TEXT NOT NULL DEFAULT '',
    url        TEXT NOT NULL DEFAULT '',
    rating     INTEGER NOT NULL DEFAULT 5,
    written    TEXT NOT NULL DEFAULT '',
    home       INTEGER NOT NULL DEFAULT 0,
    sort       INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT '',
    deleted_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE review_coaches (
    review_id TEXT NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    coach_id  TEXT NOT NULL REFERENCES coaches(id) ON DELETE CASCADE,
    PRIMARY KEY (review_id, coach_id)
);

CREATE TABLE classes (
    id            TEXT PRIMARY KEY,
    slug          TEXT NOT NULL UNIQUE,
    sort          INTEGER NOT NULL DEFAULT 0,
    visible       INTEGER NOT NULL DEFAULT 1,
    coach_id      TEXT NOT NULL DEFAULT '',
    name_en       TEXT NOT NULL DEFAULT '',
    name_ru       TEXT NOT NULL DEFAULT '',
    name_ka       TEXT NOT NULL DEFAULT '',
    desc_en       TEXT NOT NULL DEFAULT '',
    desc_ru       TEXT NOT NULL DEFAULT '',
    desc_ka       TEXT NOT NULL DEFAULT '',
    price         INTEGER NOT NULL DEFAULT 0,
    price_note_en TEXT NOT NULL DEFAULT '',
    price_note_ru TEXT NOT NULL DEFAULT '',
    price_note_ka TEXT NOT NULL DEFAULT '',
    langs         TEXT NOT NULL DEFAULT '',
    updated_at    TEXT NOT NULL DEFAULT '',
    deleted_at    TEXT NOT NULL DEFAULT ''
);

-- The weekly grid in the admin edits these rows directly (BRIEF.md §4).
CREATE TABLE class_slots (
    id       TEXT PRIMARY KEY,
    class_id TEXT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    day      INTEGER NOT NULL,
    start    TEXT NOT NULL,
    minutes  INTEGER NOT NULL DEFAULT 60
);
CREATE INDEX idx_class_slots ON class_slots(class_id, day, start);

CREATE TABLE massage (
    id         TEXT PRIMARY KEY,
    sort       INTEGER NOT NULL DEFAULT 0,
    visible    INTEGER NOT NULL DEFAULT 1,
    name_en    TEXT NOT NULL DEFAULT '',
    name_ru    TEXT NOT NULL DEFAULT '',
    name_ka    TEXT NOT NULL DEFAULT '',
    minutes    INTEGER NOT NULL DEFAULT 0,
    price      INTEGER NOT NULL DEFAULT 0,
    pack_count INTEGER NOT NULL DEFAULT 0,
    pack_price INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT '',
    deleted_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE hours (
    day        INTEGER PRIMARY KEY,
    opens      TEXT NOT NULL DEFAULT '',
    closes     TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE special_days (
    day     TEXT PRIMARY KEY,
    opens   TEXT NOT NULL DEFAULT '',
    closes  TEXT NOT NULL DEFAULT '',
    note_en TEXT NOT NULL DEFAULT '',
    note_ru TEXT NOT NULL DEFAULT '',
    note_ka TEXT NOT NULL DEFAULT ''
);

-- Content date per locale-independent path; feeds sitemap lastmod,
-- JSON-LD dateModified and Last-Modified (checklist §6: one timestamp,
-- three outputs).
CREATE TABLE page_revisions (
    page       TEXT PRIMARY KEY,
    updated_at TEXT NOT NULL
);

-- Who changed what, for recoverability (checklist §13).
CREATE TABLE changelog (
    id        TEXT PRIMARY KEY,
    at        TEXT NOT NULL,
    actor     TEXT NOT NULL DEFAULT '',
    entity    TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    action    TEXT NOT NULL,
    summary   TEXT NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE changelog;
DROP TABLE page_revisions;
DROP TABLE special_days;
DROP TABLE hours;
DROP TABLE massage;
DROP TABLE class_slots;
DROP TABLE classes;
DROP TABLE review_coaches;
DROP TABLE reviews;
DROP TABLE slug_history;
DROP TABLE coach_langs;
DROP TABLE coach_tags;
DROP TABLE coaches;
DROP TABLE tags;
DROP TABLE membership_terms;
DROP TABLE memberships;
DROP TABLE settings;
