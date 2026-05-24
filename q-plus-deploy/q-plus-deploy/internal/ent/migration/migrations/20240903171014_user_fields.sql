-- Modify "users" table
ALTER TABLE "users"
    ADD COLUMN "surname"    character varying NOT NULL default '',
    ADD COLUMN "patronymic" character varying NOT NULL default '',
    ADD COLUMN "group"      character varying NOT NULL default '',
    ADD COLUMN "gmail"      character varying NOT NULL default '';
