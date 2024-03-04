#!/bin/bash
set -e

if [[ -z "${BASELINE_MIGRATION}" ]]; then
  cd /app
  if [[ "${ALLOW_DIRTY}" == "true" ]]; then
    atlas migrate apply --url $DATABASE_URL --allow-dirty
  else
    atlas migrate apply --url $DATABASE_URL
  fi
  ./backend
else
  cd /app
  atlas migrate apply --url $DATABASE_URL --baseline $BASELINE_MIGRATION
  ./backend
fi