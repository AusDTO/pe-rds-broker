#!/bin/bash
set -e

usage() {
    cat <<EOF
Usage: $0 <create|drop>

Note: This script assumes the current user can create databases without a password.
If not, you may need to run something like

    sudo -u postgres createuser --createdb $(whoami)
    echo "CREATE USER '$(whoami)'@'localhost'; GRANT ALL PRIVILEGES ON *.* TO '$(whoami)'@'localhost' WITH GRANT OPTION;" | mysql -u root -p

or modify the script to suit your setup
EOF
}

OP=$1
if [[ -z $OP ]]; then
    usage
    exit 1
fi

DBNAME=rdsbroker

if [[ $OP == "create" ]]; then
    createdb $DBNAME
    echo "postgres database $DBNAME created"
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
    if which mysql > /dev/null; then
        echo "CREATE DATABASE $DBNAME" | mysql
        echo "mysql database $DBNAME created"
        cat >> db.env <<EOF
export RDSBROKER_SHARED_MYSQL_DB_NAME=$DBNAME
export RDSBROKER_SHARED_MYSQL_DB_URL=localhost
export RDSBROKER_SHARED_MYSQL_DB_PORT=3306
export RDSBROKER_SHARED_MYSQL_DB_USERNAME=$(whoami)
export RDSBROKER_SHARED_MYSQL_DB_PASSWORD=
export RDSBROKER_SHARED_MYSQL_DB_SSLMODE=disable
EOF
    else
        echo "skipping mysql shared database: mysql not found"
    fi
    cat <<EOF

Environment variables written to db.env

Note: By default, the broker will connect to your database using TCP. If you
wish to use unix sockets, change RDSBROKER_*_DB_URL to the correct socket
directory for your system (probably either /tmp or /var/run/postgresql for
postgres and /tmp/mysql.sock or /var/run/mysqld/mysqld.sock for mysql).

EOF
elif [[ $OP == "drop" ]]; then
    dropdb $DBNAME
    echo "postgres database $DBNAME dropped"
    if which mysql > /dev/null; then
        echo "DROP DATABASE $DBNAME" | mysql
        echo "mysql database $DBNAME dropped"
    fi
else
    usage
    exit 1
fi
