#!/bin/bash

go generate ./internal/generated || exit 1

git apply ./scripts/fix_generation.patch || exit 1

cd frontend/q_plus_front || exit 1

npm run generate-api
