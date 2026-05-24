-- Create "discord_roles" table
CREATE TABLE "discord_roles"
(
    "id"                           character varying NOT NULL,
    "type"                         character varying NOT NULL,
    "discord_server_discord_roles" character varying NOT NULL,
    PRIMARY KEY ("id"),
    CONSTRAINT "discord_roles_discord_servers_discord_roles" FOREIGN KEY ("discord_server_discord_roles") REFERENCES "discord_servers" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
