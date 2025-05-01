#!/bin/bash
set -e
serve --node-id host01  --object-store file --data-dir /var/lib/influxdb3 --without-auth

until curl http://influxdb:8181/ping 2>/dev/null; do
  echo "Waiting for InfluxDB..."
  sleep 1
done

echo "influx_db started. creating user and database"


create database metric_db

echo "InfluxDB setup complete."
