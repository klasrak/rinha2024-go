-- DB Version: 16
-- OS Type: linux
-- DB Type: oltp
-- Total Memory (RAM): 400 MB
-- CPUs num: 1
-- Connections num: 400
-- Data Storage: ssd

ALTER SYSTEM SET
 max_connections = '400';
ALTER SYSTEM SET
 shared_buffers = '100MB';
ALTER SYSTEM SET
 effective_cache_size = '300MB';
ALTER SYSTEM SET
 maintenance_work_mem = '25MB';
ALTER SYSTEM SET
 checkpoint_completion_target = '0.9';
ALTER SYSTEM SET
 wal_buffers = '3MB';
ALTER SYSTEM SET
 default_statistics_target = '100';
ALTER SYSTEM SET
 random_page_cost = '1.1';
ALTER SYSTEM SET
 effective_io_concurrency = '200';
ALTER SYSTEM SET
 work_mem = '128kB';
ALTER SYSTEM SET
 huge_pages = 'off';
ALTER SYSTEM SET
 min_wal_size = '2GB';
ALTER SYSTEM SET
 max_wal_size = '8GB';


CREATE TABLE IF NOT EXISTS users (
    "id" SERIAL PRIMARY KEY,
    "name" VARCHAR(20) NOT NULL,
    "limit" INT NOT NULL,
    "balance" INT DEFAULT 0
);

CREATE UNLOGGED TABLE transactions (
    "id" SERIAL PRIMARY KEY,
    "value" INT NOT NULL,
    "type" VARCHAR(1) NOT NULL,
    "description" VARCHAR(10) NOT NULL,
    "created_at" TIMESTAMP NOT NULL DEFAULT NOW(),
    "user_id" INT NOT NULL REFERENCES users (id)
);


DO $$
BEGIN
INSERT INTO users (name, "limit")
VALUES ('Thorin', 100000),
    ('Balin', 80000),
    ('Dwalin', 1000000),
    ('Fili', 10000000),
    ('Kili',500000);
END; $$
