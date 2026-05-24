#!/bin/bash

# Define variables
REMOTE_SERVER="pavel@2828.ftp.sh"  # Replace with your remote server's username and IP/hostname
REMOTE_BACKUP_DIR="/home/pavel/q-plus/backups"  # The path to the backup directory on the remote server
LOCAL_BACKUP_DIR="backups"  # The local directory where backups will be downloaded

# Step 1: Ensure the local backup directory exists
echo "Ensuring local backup directory exists at $LOCAL_BACKUP_DIR..."
mkdir -p "$LOCAL_BACKUP_DIR"

# Step 2: Download the backup directory from the remote server using rsync
echo "Downloading backup from $REMOTE_SERVER:$REMOTE_BACKUP_DIR to $LOCAL_BACKUP_DIR..."
rsync -avz --rsync-path="sudo rsync" --progress "$REMOTE_SERVER:$REMOTE_BACKUP_DIR/" "$LOCAL_BACKUP_DIR/$REMOTE_SERVER/"

echo "Backup download complete!"
