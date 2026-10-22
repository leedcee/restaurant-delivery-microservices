FROM migrate/migrate:v4.18.3

COPY deploy/staging/migrate-entrypoint.sh /migrate-entrypoint.sh
USER nobody
ENTRYPOINT ["/bin/sh", "/migrate-entrypoint.sh"]
CMD ["up"]
