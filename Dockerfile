FROM python:3.11-alpine

COPY entrypoint.sh /entrypoint.sh
COPY code /code
RUN pip install -q -r /code/requirements.txt

ENTRYPOINT ["/entrypoint.sh"]