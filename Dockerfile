FROM python:3.11-bullseye

ENV USAGE_LAMBDA_TOKEN=diggerABC@@1998fE

COPY entrypoint.sh /entrypoint.sh
COPY code /code
RUN pip install -q -r /code/requirements.txt

ENTRYPOINT ["/entrypoint.sh"]
