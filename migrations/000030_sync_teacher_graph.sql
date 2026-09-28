-- +goose Up
ALTER TABLE authz_outbox DROP CONSTRAINT authz_outbox_operation_check;
ALTER TABLE authz_outbox ADD CONSTRAINT authz_outbox_operation_check CHECK(operation IN
    ('TOUCH','DELETE','DELETE_BY_OBJECT','DELETE_BY_SUBJECT','REPLACE','SYNC_DEPARTMENT_STAFF'));
-- Department staff is derived from active teacher positions.
-- The worker reads current state when processing these refresh requests.
-- +goose StatementBegin
CREATE FUNCTION enqueue_teacher_department(dept BIGINT) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO authz_outbox(operation, payload)
    SELECT 'SYNC_DEPARTMENT_STAFF', jsonb_build_object('object_type','department','object_id',code,'relation','staff')
    FROM departments WHERE id=dept;
    PERFORM pg_notify('authz_outbox','');
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION refresh_teacher_position_graph() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP <> 'INSERT' THEN
        PERFORM enqueue_teacher_department(OLD.department_id);
    END IF;
    IF TG_OP <> 'DELETE' THEN
        PERFORM enqueue_teacher_department(NEW.department_id);
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER teacher_position_graph AFTER INSERT OR UPDATE OR DELETE ON teacher_positions
FOR EACH ROW EXECUTE FUNCTION refresh_teacher_position_graph();

-- +goose StatementBegin
CREATE FUNCTION refresh_teacher_person_graph() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE dept BIGINT;
BEGIN
    FOR dept IN SELECT DISTINCT department_id FROM teacher_positions WHERE person_id=NEW.id LOOP
        PERFORM enqueue_teacher_department(dept);
    END LOOP;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER teacher_person_graph AFTER UPDATE OF external_id ON persons
FOR EACH ROW WHEN (OLD.external_id IS DISTINCT FROM NEW.external_id)
EXECUTE FUNCTION refresh_teacher_person_graph();

INSERT INTO authz_outbox(operation, payload)
SELECT 'SYNC_DEPARTMENT_STAFF', jsonb_build_object('object_type','department','object_id',code,'relation','staff')
FROM departments;
SELECT pg_notify('authz_outbox','');

-- +goose Down
DROP TRIGGER IF EXISTS teacher_person_graph ON persons;
DROP TRIGGER IF EXISTS teacher_position_graph ON teacher_positions;
DROP FUNCTION IF EXISTS refresh_teacher_person_graph();
DROP FUNCTION IF EXISTS refresh_teacher_position_graph();
DROP FUNCTION IF EXISTS enqueue_teacher_department(BIGINT);

DELETE FROM authz_outbox WHERE operation='SYNC_DEPARTMENT_STAFF';
ALTER TABLE authz_outbox DROP CONSTRAINT authz_outbox_operation_check;
ALTER TABLE authz_outbox ADD CONSTRAINT authz_outbox_operation_check CHECK(operation IN
    ('TOUCH','DELETE','DELETE_BY_OBJECT','DELETE_BY_SUBJECT','REPLACE'));
