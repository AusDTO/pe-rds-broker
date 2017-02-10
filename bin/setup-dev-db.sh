#!/bin/bash
set -e

OP=$1
if [[ -z $OP ]]; then
    cat <<EOF
Usage: $0 <create|drop>

Note: This script assumes the current user can create databases.
If not, you may need to run something like

    sudo -u postgres createuser --createdb $(whoami)

EOF
    exit 1
fi

DBNAME=rdsbroker

if [[ $OP == "create" ]]; then
    createdb $DBNAME
    cat > db.env <<EOF
export RDSBROKER_INTERNAL_DB_PROVIDER=postgres
export RDSBROKER_INTERNAL_DB_NAME=$DBNAME
export RDSBROKER_INTERNAL_DB_URL=localhost
export RDSBROKER_INTERNAL_DB_PORT=5432
export RDSBROKER_INTERNAL_DB_USERNAME=$(whoami)
export RDSBROKER_INTERNAL_DB_PASSWORD=
export RDSBROKER_INTERNAL_DB_SSLMODE=disable
export RDSBROKER_SHARED_POSTGRES_DB_NAME=$DBNAME
export RDSBROKER_SHARED_POSTGRES_DB_URL=localhost
export RDSBROKER_SHARED_POSTGRES_DB_PORT=5432
export RDSBROKER_SHARED_POSTGRES_DB_USERNAME=$(whoami)
export RDSBROKER_SHARED_POSTGRES_DB_PASSWORD=
export RDSBROKER_SHARED_POSTGRES_DB_SSLMODE=disable
EOF
    cat <<EOF
Database $DBNAME created
Environment variables written to db.env

Note: By default, the broker will connect to your database using TCP. If you
wish to use unix sockets, change RDSBROKER_*_DB_URL to the correct
socket directory for your system (probably either /tmp or /var/run/postgresql).

EOF
elif [[ $OP == "drop" ]]; then
    dropdb $DBNAME
    echo "Database $DBNAME dropped"
fi
