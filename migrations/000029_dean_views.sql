-- +goose Up

-- Full student card with hierarchy
CREATE VIEW v_students AS
SELECT
    p.id              AS person_id,
    p.external_id,
    p.last_name,
    p.first_name,
    p.middle_name,
    p.email,
    p.phone,
    p.global_user_id  AS bot_user_id,
    sp.id             AS position_id,
    sp.status,
    sp.nationality_type,
    sp.funding_type,
    sp.education_form,
    sp.enrolled_at,
    sp.graduated_at,
    f.id              AS faculty_id,
    f.name            AS faculty_name,
    f.code            AS faculty_code,
    d.id              AS department_id,
    d.name            AS department_name,
    pr.id             AS program_id,
    pr.name           AS program_name,
    pr.degree_level,
    st.id             AS stream_id,
    st.code           AS stream_code,
    sg.id             AS group_id,
    sg.code           AS group_code,
    sg.name           AS group_name
FROM persons p
JOIN student_positions sp ON sp.person_id = p.id
LEFT JOIN study_groups sg ON sg.id = sp.study_group_id
LEFT JOIN streams      st ON st.id = sg.stream_id
LEFT JOIN programs     pr ON pr.id = st.program_id
LEFT JOIN departments  d  ON d.id  = pr.department_id
LEFT JOIN faculties    f  ON f.id  = d.faculty_id;

-- Per-faculty statistics
CREATE VIEW v_faculty_stats AS
SELECT
    f.id                                                                          AS faculty_id,
    f.name                                                                        AS faculty_name,
    f.code                                                                        AS faculty_code,
    COUNT(DISTINCT sg.id)                                                         AS group_count,
    COUNT(DISTINCT sp.id) FILTER (WHERE sp.status = 'active')                    AS active_students,
    COUNT(DISTINCT sp.id) FILTER (WHERE sp.status = 'active' AND sp.funding_type = 'budget')   AS budget_students,
    COUNT(DISTINCT sp.id) FILTER (WHERE sp.status = 'active' AND sp.funding_type = 'contract') AS contract_students,
    COUNT(DISTINCT sp.id) FILTER (WHERE sp.status = 'active' AND sp.nationality_type = 'foreign') AS foreign_students
FROM faculties f
LEFT JOIN departments  d  ON d.faculty_id  = f.id
LEFT JOIN programs     pr ON pr.department_id = d.id
LEFT JOIN streams      st ON st.program_id    = pr.id
LEFT JOIN study_groups sg ON sg.stream_id     = st.id
LEFT JOIN student_positions sp ON sp.study_group_id = sg.id
GROUP BY f.id, f.name, f.code;

-- Per-group statistics
CREATE VIEW v_group_stats AS
SELECT
    sg.id                                                                          AS group_id,
    sg.code                                                                        AS group_code,
    sg.name                                                                        AS group_name,
    st.id                                                                          AS stream_id,
    pr.id                                                                          AS program_id,
    pr.name                                                                        AS program_name,
    d.id                                                                           AS department_id,
    f.id                                                                           AS faculty_id,
    COUNT(sp.id) FILTER (WHERE sp.status = 'active')                              AS active_students,
    COUNT(sp.id) FILTER (WHERE sp.status = 'active' AND sp.funding_type = 'budget')   AS budget_students,
    COUNT(sp.id) FILTER (WHERE sp.status = 'active' AND sp.funding_type = 'contract') AS contract_students,
    COUNT(sp.id) FILTER (WHERE sp.status = 'active' AND sp.nationality_type = 'foreign') AS foreign_students
FROM study_groups sg
JOIN streams      st ON st.id = sg.stream_id
JOIN programs     pr ON pr.id = st.program_id
JOIN departments  d  ON d.id  = pr.department_id
JOIN faculties    f  ON f.id  = d.faculty_id
LEFT JOIN student_positions sp ON sp.study_group_id = sg.id
GROUP BY sg.id, sg.code, sg.name, st.id, pr.id, pr.name, d.id, f.id;

-- +goose Down
DROP VIEW IF EXISTS v_group_stats;
DROP VIEW IF EXISTS v_faculty_stats;
DROP VIEW IF EXISTS v_students;
