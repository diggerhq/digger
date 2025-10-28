select
    cron.schedule(
            'invoke-drift-schedule-every-hour-00',
            '0 * * * *',
            $$
                select
      net.http_post(
          url:='https://{DIGGER_HOSTNAME}/_internal/process_drift',
          headers:=jsonb_build_object('Content-Type','application/json', 'Authorization', 'Bearer ' || {DIGGER_WEBHOOK_SECRET}),
          body:=jsonb_build_object('time', now() ),
          timeout_milliseconds:=5000
      ) as request_id;
$$
);
