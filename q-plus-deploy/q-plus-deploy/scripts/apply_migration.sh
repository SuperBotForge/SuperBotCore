#!/bin/bash

atlas migrate apply \
  --dir "file://internal/ent/migration/migrations" \
  --url "postgres://$DB_USER:$DB_PASSWORD@$DB_HOST:$DB_PORT/$DB_NAME?search_path=public&sslmode=disable"
#  --baseline "20240813145349"
