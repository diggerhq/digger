#!/bin/bash
set -e

if [[ -z "${BASELINE_MIGRATION}" ]]; then
  cd /app
  atlas migrate apply --url $DATABASE_URL
  ./backend
else
  cd /app
  atlas migrate apply --url $DATABASE_URL --baseline $BASELINE_MIGRATION
  ./backend
fi