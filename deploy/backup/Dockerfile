FROM postgres:17-alpine

RUN apk add --no-cache age bash

COPY scripts/backup-postgres.sh /usr/local/bin/backup-postgres.sh
COPY scripts/restore-postgres.sh /usr/local/bin/restore-postgres.sh
COPY scripts/backup-restore-drill.sh /usr/local/bin/backup-restore-drill.sh
RUN chmod 0555 /usr/local/bin/backup-postgres.sh /usr/local/bin/restore-postgres.sh /usr/local/bin/backup-restore-drill.sh

USER postgres
ENTRYPOINT ["/usr/local/bin/backup-restore-drill.sh"]
