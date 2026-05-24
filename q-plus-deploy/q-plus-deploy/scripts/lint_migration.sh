#!/bin/bash

atlas migrate lint \
  --dir "file://internal/ent/migration/migrations" \
  --dev-url="docker://postgres/16/test?search_path=public" \
  --latest 1
