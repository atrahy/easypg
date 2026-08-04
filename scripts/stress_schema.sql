-- Stress schema for exercising the easypg Definition Tab pane limits.
--
-- Creates a large number of schemas, tables, views, columns, indexes,
-- constraints and functions to test the scrolling / rendering limits of the
-- three panes:
--
--   ~50 schemas                 -> schema pane scroll (only ~6 rows visible)
--   1 schema with 50 tables     -> objects pane, Table tab scroll
--   1 schema with 50 views      -> objects pane, View tab scroll
--   1 schema with 50 functions  -> objects pane, Function tab scroll (lazy load)
--   1 table with 50 columns     -> detail Column tab scroll
--   1 table with 50 indexes     -> detail Index tab scroll
--   1 table with 20 constraints -> detail Constraints tab scroll
--
-- Independent from example_schema.sql: every object lives under a `stress_`
-- prefix, and the script drops/recreates those schemas first, so it is
-- re-runnable and never touches hr/sales/public.
--
-- Run with:
--   psql "postgres://local_user@localhost:5432/local_db" -f scripts/stress_schema.sql

-- ---------------------------------------------------------------------------
-- Cleanup: drop every schema this script owns (prefix `stress_`) so a re-run
-- starts from a clean slate. The backslash escapes LIKE's `_` wildcard.
-- ---------------------------------------------------------------------------
DO $do$
DECLARE
    s text;
BEGIN
    FOR s IN
        SELECT nspname FROM pg_catalog.pg_namespace WHERE nspname LIKE 'stress\_%'
    LOOP
        EXECUTE format('DROP SCHEMA IF EXISTS %I CASCADE', s);
    END LOOP;
END;
$do$;

-- ---------------------------------------------------------------------------
-- 50 schemas: stress_s01 .. stress_s50, each with a few tables and a view, so
-- moving the schema cursor keeps triggering fetches with non-empty content.
-- ---------------------------------------------------------------------------
DO $do$
DECLARE
    i     int;
    j     int;
    sname text;
    tname text;
BEGIN
    FOR i IN 1..50 LOOP
        sname := 'stress_s' || lpad(i::text, 2, '0');
        EXECUTE format('CREATE SCHEMA %I', sname);

        FOR j IN 1..3 LOOP
            tname := 'table_' || lpad(j::text, 2, '0');
            EXECUTE format(
                'CREATE TABLE %I.%I (
                    id         serial PRIMARY KEY,
                    label      text NOT NULL,
                    created_at timestamptz NOT NULL DEFAULT now()
                )',
                sname, tname
            );
        END LOOP;

        EXECUTE format(
            'CREATE VIEW %I.summary AS SELECT id, label FROM %I.table_01',
            sname, sname
        );
    END LOOP;
END;
$do$;

-- ---------------------------------------------------------------------------
-- One wide schema: 50 tables + 50 views + 50 functions, to stress each of the
-- objects pane's internal tabs (Table / View / Function).
-- ---------------------------------------------------------------------------
CREATE SCHEMA stress_wide_objects;

DO $do$
DECLARE
    i     int;
    oname text;
BEGIN
    FOR i IN 1..50 LOOP
        oname := 'obj_' || lpad(i::text, 2, '0');
        EXECUTE format(
            'CREATE TABLE stress_wide_objects.%I (
                id     serial PRIMARY KEY,
                name   text NOT NULL,
                amount numeric(12,2) NOT NULL DEFAULT 0
            )',
            oname
        );

        EXECUTE format(
            'CREATE VIEW stress_wide_objects.%I AS SELECT id, name FROM stress_wide_objects.%I',
            'view_' || lpad(i::text, 2, '0'), oname
        );

        EXECUTE format(
            'CREATE FUNCTION stress_wide_objects.%I(p integer) RETURNS integer
                LANGUAGE sql IMMUTABLE AS $fn$ SELECT p + %s $fn$',
            'fn_' || lpad(i::text, 2, '0'), i
        );
    END LOOP;
END;
$do$;

-- ---------------------------------------------------------------------------
-- One table with 50 columns of varied types, to stress the detail Column tab.
-- ---------------------------------------------------------------------------
CREATE SCHEMA stress_wide_columns;

DO $do$
DECLARE
    i     int;
    cols  text := '';
    types text[] := ARRAY[
        'integer', 'text', 'numeric(12,2)', 'boolean', 'timestamptz',
        'date', 'uuid', 'jsonb', 'bigint', 'real'
    ];
BEGIN
    FOR i IN 1..50 LOOP
        cols := cols
            || format('%I %s NOT NULL',
                      'col_' || lpad(i::text, 2, '0'),
                      types[1 + (i % array_length(types, 1))]);
        IF i < 50 THEN
            cols := cols || ', ';
        END IF;
    END LOOP;

    EXECUTE format(
        'CREATE TABLE stress_wide_columns.big_table (id serial PRIMARY KEY, %s)',
        cols
    );
END;
$do$;

-- ---------------------------------------------------------------------------
-- One table with 50 indexes and 20 check constraints, to stress the detail
-- Index and Constraints tabs.
-- ---------------------------------------------------------------------------
CREATE SCHEMA stress_wide_meta;

DO $do$
DECLARE
    i    int;
    cols text := '';
    cnam text;
BEGIN
    FOR i IN 1..50 LOOP
        cols := cols || format('%I integer NOT NULL DEFAULT 0', 'c' || lpad(i::text, 2, '0'));
        IF i < 50 THEN
            cols := cols || ', ';
        END IF;
    END LOOP;

    EXECUTE format('CREATE TABLE stress_wide_meta.indexed (id serial PRIMARY KEY, %s)', cols);

    FOR i IN 1..50 LOOP
        cnam := 'c' || lpad(i::text, 2, '0');
        EXECUTE format(
            'CREATE INDEX %I ON stress_wide_meta.indexed (%I)',
            'idx_' || lpad(i::text, 2, '0'), cnam
        );
    END LOOP;

    FOR i IN 1..20 LOOP
        cnam := 'c' || lpad(i::text, 2, '0');
        EXECUTE format(
            'ALTER TABLE stress_wide_meta.indexed ADD CONSTRAINT %I CHECK (%I >= 0)',
            'chk_' || lpad(i::text, 2, '0'), cnam
        );
    END LOOP;
END;
$do$;
