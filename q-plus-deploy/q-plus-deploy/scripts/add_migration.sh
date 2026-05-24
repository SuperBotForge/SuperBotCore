#!/bin/bash

if [ -z "$1" ]
then
    echo "Please provide a name for a migration"
    exit 1
fi

echo "Creating a new migration named $1"

atlas migrate diff "$1" \
  --dir "file://internal/ent/migration/migrations" \
  --to "ent://internal/ent/schema" \
  --dev-url "docker://postgres/16/test?search_path=public"

