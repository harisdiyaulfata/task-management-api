-- Preserve the existing UUID values as public API identifiers while moving all
-- relational primary/foreign keys to compact BIGINT values.

CREATE SEQUENCE users_id_seq;
CREATE SEQUENCE tasks_id_seq;
CREATE SEQUENCE task_logs_id_seq;

ALTER TABLE users ADD COLUMN internal_id BIGINT;
UPDATE users SET internal_id = nextval('users_id_seq');
ALTER TABLE users ALTER COLUMN internal_id SET NOT NULL;
ALTER TABLE users ALTER COLUMN internal_id SET DEFAULT nextval('users_id_seq');
ALTER SEQUENCE users_id_seq OWNED BY users.internal_id;
SELECT setval('users_id_seq', COALESCE((SELECT MAX(internal_id) FROM users), 1), true);

ALTER TABLE tasks ADD COLUMN internal_id BIGINT;
ALTER TABLE tasks ADD COLUMN owner_internal_id BIGINT;
ALTER TABLE tasks ADD COLUMN assignee_internal_id BIGINT;
UPDATE tasks SET internal_id = nextval('tasks_id_seq');
UPDATE tasks t SET owner_internal_id = u.internal_id FROM users u WHERE t.owner_id = u.id;
UPDATE tasks t SET assignee_internal_id = u.internal_id FROM users u WHERE t.assignee_id = u.id;
ALTER TABLE tasks ALTER COLUMN internal_id SET NOT NULL;
ALTER TABLE tasks ALTER COLUMN owner_internal_id SET NOT NULL;
ALTER TABLE tasks ALTER COLUMN internal_id SET DEFAULT nextval('tasks_id_seq');
ALTER SEQUENCE tasks_id_seq OWNED BY tasks.internal_id;
SELECT setval('tasks_id_seq', COALESCE((SELECT MAX(internal_id) FROM tasks), 1), true);

ALTER TABLE task_logs ADD COLUMN internal_id BIGINT;
ALTER TABLE task_logs ADD COLUMN task_internal_id BIGINT;
ALTER TABLE task_logs ADD COLUMN actor_internal_id BIGINT;
UPDATE task_logs SET internal_id = nextval('task_logs_id_seq');
UPDATE task_logs l SET task_internal_id = t.internal_id FROM tasks t WHERE l.task_id = t.id;
UPDATE task_logs l SET actor_internal_id = u.internal_id FROM users u WHERE l.actor_id = u.id;
ALTER TABLE task_logs ALTER COLUMN internal_id SET NOT NULL;
ALTER TABLE task_logs ALTER COLUMN task_internal_id SET NOT NULL;
ALTER TABLE task_logs ALTER COLUMN actor_internal_id SET NOT NULL;
ALTER TABLE task_logs ALTER COLUMN internal_id SET DEFAULT nextval('task_logs_id_seq');
ALTER SEQUENCE task_logs_id_seq OWNED BY task_logs.internal_id;
SELECT setval('task_logs_id_seq', COALESCE((SELECT MAX(internal_id) FROM task_logs), 1), true);

ALTER TABLE users ADD COLUMN public_id UUID;
UPDATE users SET public_id = id;
ALTER TABLE users ALTER COLUMN public_id SET NOT NULL;
ALTER TABLE users ALTER COLUMN public_id SET DEFAULT gen_random_uuid();
ALTER TABLE users ADD CONSTRAINT users_public_id_key UNIQUE (public_id);

ALTER TABLE tasks ADD COLUMN public_id UUID;
UPDATE tasks SET public_id = id;
ALTER TABLE tasks ALTER COLUMN public_id SET NOT NULL;
ALTER TABLE tasks ALTER COLUMN public_id SET DEFAULT gen_random_uuid();
ALTER TABLE tasks ADD CONSTRAINT tasks_public_id_key UNIQUE (public_id);

DROP INDEX IF EXISTS idx_tasks_owner_id;
DROP INDEX IF EXISTS idx_tasks_assignee_id;
DROP INDEX IF EXISTS idx_tasks_owner_status;

ALTER TABLE task_logs DROP CONSTRAINT task_logs_task_id_fkey;
ALTER TABLE task_logs DROP CONSTRAINT task_logs_actor_id_fkey;
ALTER TABLE tasks DROP CONSTRAINT tasks_owner_id_fkey;
ALTER TABLE tasks DROP CONSTRAINT tasks_assignee_id_fkey;
ALTER TABLE task_logs DROP CONSTRAINT task_logs_pkey;
ALTER TABLE tasks DROP CONSTRAINT tasks_pkey;
ALTER TABLE users DROP CONSTRAINT users_pkey;

ALTER TABLE task_logs DROP COLUMN task_id;
ALTER TABLE task_logs DROP COLUMN actor_id;
ALTER TABLE task_logs DROP COLUMN id;
ALTER TABLE tasks DROP COLUMN owner_id;
ALTER TABLE tasks DROP COLUMN assignee_id;
ALTER TABLE tasks DROP COLUMN id;
ALTER TABLE users DROP COLUMN id;

ALTER TABLE users RENAME COLUMN internal_id TO id;
ALTER TABLE tasks RENAME COLUMN internal_id TO id;
ALTER TABLE tasks RENAME COLUMN owner_internal_id TO owner_id;
ALTER TABLE tasks RENAME COLUMN assignee_internal_id TO assignee_id;
ALTER TABLE task_logs RENAME COLUMN internal_id TO id;
ALTER TABLE task_logs RENAME COLUMN task_internal_id TO task_id;
ALTER TABLE task_logs RENAME COLUMN actor_internal_id TO actor_id;

ALTER TABLE users ADD PRIMARY KEY (id);
ALTER TABLE tasks ADD PRIMARY KEY (id);
ALTER TABLE task_logs ADD PRIMARY KEY (id);

ALTER TABLE tasks ADD CONSTRAINT tasks_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE tasks ADD CONSTRAINT tasks_assignee_id_fkey FOREIGN KEY (assignee_id) REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE task_logs ADD CONSTRAINT task_logs_task_id_fkey FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE;
ALTER TABLE task_logs ADD CONSTRAINT task_logs_actor_id_fkey FOREIGN KEY (actor_id) REFERENCES users(id) ON DELETE RESTRICT;

CREATE INDEX idx_tasks_owner_id ON tasks(owner_id);
CREATE INDEX idx_tasks_assignee_id ON tasks(assignee_id);
CREATE INDEX idx_tasks_owner_status ON tasks(owner_id, status);
CREATE INDEX idx_task_logs_task_id ON task_logs(task_id);
