if [[ -z "${BASELINE_MIGRATION}" ]]; then
  cd /app
  atlas migrate apply --url $DATABASE_URL --baseline $BASELINE_MIGRATION
  ./backend
else
  cd /app
  atlas migrate apply --url $DATABASE_URL
  ./backend
fi