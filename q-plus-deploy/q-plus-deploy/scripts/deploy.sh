#!/bin/bash

# Define variables
REMOTE_DOCKER_CONTEXT="ssh2828" # Name of your Docker SSH context
REMOTE_SERVER="pavel@2828.ftp.sh" # Replace with your remote server's username and IP/hostname
COMPOSE_FILE="compose.yaml" # Path to your docker-compose file (default is compose.yaml)
ENV_FILE=".env.prod" # The environment file to use
BUILD_OPTION="" # Default is no --build option

# Check for the --build argument
if [ "$1" == "--build" ]; then
  BUILD_OPTION="--build"
  echo "Build option enabled: The containers will be rebuilt."
else
  echo "Build option (--build) not provided: The containers will not be rebuilt."
fi

# Load environment variables from the .env.prod file
export $(grep -v '^#' $ENV_FILE | xargs)

# Paths for migration and backup directories (from the .env.prod file)
MIGRATION_DIR_LOCAL=./internal/ent/migration/migrations/
MIGRATION_DIR_REMOTE=$MIGRATION_DIR
BACKUP_DIR_REMOTE=$BACKUP_DIR

# Step 1: Copy migration files to the remote server
echo "Copying migration files to $REMOTE_SERVER:$MIGRATION_DIR_REMOTE..."
ssh "$REMOTE_SERVER" "mkdir -p $MIGRATION_DIR_REMOTE"
rsync -avz --delete "$MIGRATION_DIR_LOCAL" "$REMOTE_SERVER:$MIGRATION_DIR_REMOTE"

# Step 2: Create backup directory on the remote server
echo "Creating backup directory on $REMOTE_SERVER:$BACKUP_DIR_REMOTE..."
ssh "$REMOTE_SERVER" "mkdir -p $BACKUP_DIR_REMOTE"

# Step 3: Create the remote Docker context if it doesn't exist
docker context inspect $REMOTE_DOCKER_CONTEXT > /dev/null 2>&1
if [ $? -ne 0 ]; then
  echo "Creating a new Docker SSH context..."
  docker context create $REMOTE_DOCKER_CONTEXT --docker "host=ssh://$REMOTE_SERVER"
else
  echo "Using existing Docker SSH context: $REMOTE_DOCKER_CONTEXT"
fi

# Step 4: Deploy the application using the remote context and environment variables
DOCKER_CONTEXT=$REMOTE_DOCKER_CONTEXT docker compose --env-file $ENV_FILE -f $COMPOSE_FILE up -d $BUILD_OPTION

echo "Deployment complete!"
