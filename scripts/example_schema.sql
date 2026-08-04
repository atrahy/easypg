-- Example schema for exercising the easypg Definition Tab.
-- Theme: a small company. Two schemas (hr, sales), 5 tables each, with a
-- variety of constraints (PK/FK/UNIQUE/CHECK/DEFAULT/NOT NULL — including a
-- cross-schema FK and a self-referencing FK), several kinds of index
-- (composite, partial, expression/unique), 3 functions and 2 views.
--
-- No data is inserted. Assumes an empty database (no drops / IF NOT EXISTS).
-- Run with:
--   psql "postgres://local_user@localhost:5432/local_db" -f scripts/example_schema.sql

CREATE SCHEMA hr;
CREATE SCHEMA sales;

-- =====================================================================
-- Schema: hr
-- =====================================================================

CREATE TABLE hr.offices (
    id        serial PRIMARY KEY,
    name      text    NOT NULL,
    city      text    NOT NULL,
    country   text    NOT NULL DEFAULT 'France',
    capacity  integer NOT NULL DEFAULT 0 CHECK (capacity >= 0),
    opened_on date,
    CONSTRAINT offices_name_key UNIQUE (name)
);

CREATE TABLE hr.positions (
    id         serial PRIMARY KEY,
    title      text          NOT NULL UNIQUE,
    min_salary numeric(10,2) NOT NULL DEFAULT 0,
    max_salary numeric(10,2) NOT NULL,
    CONSTRAINT positions_salary_range CHECK (max_salary >= min_salary)
);

CREATE TABLE hr.departments (
    id         serial PRIMARY KEY,
    name       text          NOT NULL UNIQUE,
    office_id  integer       NOT NULL REFERENCES hr.offices(id),
    budget     numeric(14,2) NOT NULL DEFAULT 0 CHECK (budget >= 0),
    created_at timestamptz   NOT NULL DEFAULT now()
);

CREATE TABLE hr.employees (
    id            serial PRIMARY KEY,
    first_name    text          NOT NULL,
    last_name     text          NOT NULL,
    email         text          NOT NULL UNIQUE,
    department_id integer       NOT NULL REFERENCES hr.departments(id),
    position_id   integer       NOT NULL REFERENCES hr.positions(id),
    office_id     integer       NOT NULL REFERENCES hr.offices(id),
    manager_id    integer       REFERENCES hr.employees(id),
    hired_on      date          NOT NULL DEFAULT CURRENT_DATE,
    salary        numeric(10,2) NOT NULL DEFAULT 0 CHECK (salary >= 0),
    is_active     boolean       NOT NULL DEFAULT true
);

CREATE TABLE hr.employee_skills (
    employee_id integer NOT NULL REFERENCES hr.employees(id) ON DELETE CASCADE,
    skill       text    NOT NULL,
    level       integer NOT NULL DEFAULT 1 CHECK (level BETWEEN 1 AND 5),
    PRIMARY KEY (employee_id, skill)
);

-- Indexes: FK lookups, a composite, a partial, and an expression-unique index.
CREATE INDEX employees_department_idx  ON hr.employees (department_id);
CREATE INDEX employees_office_idx      ON hr.employees (office_id);
CREATE INDEX employees_last_first_idx  ON hr.employees (last_name, first_name);
CREATE INDEX employees_active_idx      ON hr.employees (is_active) WHERE is_active;
CREATE UNIQUE INDEX employees_email_lower_idx ON hr.employees (lower(email));
CREATE INDEX departments_office_idx    ON hr.departments (office_id);

-- =====================================================================
-- Schema: sales
-- =====================================================================

CREATE TABLE sales.customers (
    id         serial PRIMARY KEY,
    name       text        NOT NULL,
    email      text        NOT NULL UNIQUE,
    phone      text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sales.products (
    id       serial PRIMARY KEY,
    sku      text          NOT NULL UNIQUE,
    name     text          NOT NULL,
    price    numeric(10,2) NOT NULL CHECK (price > 0),
    in_stock integer       NOT NULL DEFAULT 0 CHECK (in_stock >= 0),
    category text
);

CREATE TABLE sales.orders (
    id          serial PRIMARY KEY,
    customer_id integer       NOT NULL REFERENCES sales.customers(id),
    employee_id integer       REFERENCES hr.employees(id),   -- cross-schema FK
    ordered_at  timestamptz   NOT NULL DEFAULT now(),
    status      text          NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'paid', 'shipped', 'cancelled')),
    total       numeric(12,2) NOT NULL DEFAULT 0
);

CREATE TABLE sales.order_items (
    id         serial PRIMARY KEY,
    order_id   integer       NOT NULL REFERENCES sales.orders(id) ON DELETE CASCADE,
    product_id integer       NOT NULL REFERENCES sales.products(id),
    quantity   integer       NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price numeric(10,2) NOT NULL CHECK (unit_price >= 0),
    CONSTRAINT order_items_unique UNIQUE (order_id, product_id)
);

CREATE TABLE sales.invoices (
    id        serial PRIMARY KEY,
    order_id  integer       NOT NULL UNIQUE REFERENCES sales.orders(id),
    issued_on date          NOT NULL DEFAULT CURRENT_DATE,
    amount    numeric(12,2) NOT NULL CHECK (amount >= 0),
    paid      boolean       NOT NULL DEFAULT false
);

CREATE INDEX orders_customer_idx    ON sales.orders (customer_id);
CREATE INDEX orders_status_idx      ON sales.orders (status);
CREATE INDEX order_items_order_idx  ON sales.order_items (order_id);
CREATE INDEX order_items_product_idx ON sales.order_items (product_id);
CREATE INDEX products_category_idx  ON sales.products (category) WHERE category IS NOT NULL;

-- =====================================================================
-- Functions
-- =====================================================================

-- hr.hire_new: insert a new employee and return its id (plpgsql, with a default arg).
CREATE FUNCTION hr.hire_new(
    p_first_name text,
    p_last_name  text,
    p_email      text,
    p_department integer,
    p_position   integer,
    p_office     integer,
    p_salary     numeric DEFAULT 40000
) RETURNS integer
LANGUAGE plpgsql
AS $$
DECLARE
    new_id integer;
BEGIN
    INSERT INTO hr.employees (first_name, last_name, email, department_id, position_id, office_id, hired_on, salary)
    VALUES (p_first_name, p_last_name, p_email, p_department, p_position, p_office, CURRENT_DATE, p_salary)
    RETURNING id INTO new_id;

    RETURN new_id;
END;
$$;

-- hr.employee_count: number of active employees in a department (sql, stable).
CREATE FUNCTION hr.employee_count(p_department integer)
RETURNS bigint
LANGUAGE sql
STABLE
AS $$
    SELECT count(*)
    FROM hr.employees
    WHERE department_id = p_department AND is_active;
$$;

-- sales.order_total: sum of an order's line items (sql, stable).
CREATE FUNCTION sales.order_total(p_order integer)
RETURNS numeric
LANGUAGE sql
STABLE
AS $$
    SELECT COALESCE(sum(quantity * unit_price), 0)
    FROM sales.order_items
    WHERE order_id = p_order;
$$;

-- =====================================================================
-- Views
-- =====================================================================

CREATE VIEW hr.active_employees AS
SELECT e.id,
       e.first_name,
       e.last_name,
       e.email,
       d.name  AS department,
       o.city  AS office_city,
       e.salary
FROM hr.employees e
JOIN hr.departments d ON d.id = e.department_id
JOIN hr.offices o     ON o.id = e.office_id
WHERE e.is_active;

CREATE VIEW sales.order_summary AS
SELECT o.id,
       c.name AS customer,
       o.ordered_at,
       o.status,
       sales.order_total(o.id) AS total
FROM sales.orders o
JOIN sales.customers c ON c.id = o.customer_id;
