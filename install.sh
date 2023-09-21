#!/usr/bin/env bash

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  echo "Please run as root."
  exit 1
fi

# stop the current nvidler service
systemctl stop nvidler || true

# Create directory for the binary if it doesn't exist
mkdir -p /usr/local/bin

# Copy the compiled Go binary to /usr/local/bin
cp nvidler /usr/local/bin/

# Set the executable permission
chmod +x /usr/local/bin/nvidler

# Copy the systemd service file to /etc/systemd/system
cp nvidler.service /etc/systemd/system/

# Reload systemd to recognize the new service
systemctl daemon-reload

# Enable the service to start on boot
systemctl enable nvidler

# Start the service
systemctl start nvidler

# Output the status of the service
systemctl status nvidler
