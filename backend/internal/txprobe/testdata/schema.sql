-- Transaction fixture template. testdb.Open qualifies these objects with a
-- per-test schema so pooled connections cannot change the test's search path.
CREATE TABLE __SCHEMA__.products (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE __SCHEMA__.events (
  id BIGSERIAL PRIMARY KEY,
  product_name TEXT NOT NULL,
  event_key TEXT NOT NULL,
  CONSTRAINT events_event_key_unique UNIQUE (event_key) DEFERRABLE INITIALLY DEFERRED
);
