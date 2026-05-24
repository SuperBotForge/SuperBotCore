#!/bin/bash

# cd to ./internal and run `go run -mod=mod entgo.io/ent/cmd/ent new $1`
# $1 is the name of the ent schema
# This script will create a new ent schema with the name provided
# Example: ./scripts/ent_new.sh User
# This will create a new ent schema named "User"

# Check if the user provided a name for the ent schema
if [ -z "$1" ]
then
    echo "Please provide a name for the ent schema"
    exit 1
fi

echo "Creating a new ent schema named $1"
cd internal || exit
go run -mod=mod entgo.io/ent/cmd/ent new "$1" || exit
echo "internal/ent/schema/$1.go created successfully"
