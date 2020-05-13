FROM golang:alpine
RUN apk add bash git
RUN go get -tags 'postgres' -u github.com/golang-migrate/migrate/cmd/migrate
ADD run_db_migrations.sh /run_db_migrations.sh
RUN wget https://dl.google.com/cloudsql/cloud_sql_proxy.linux.amd64 -O /bin/cloud_sql_proxy
RUN chmod +x /bin/cloud_sql_proxy
ENTRYPOINT ["/run_db_migrations.sh"]
