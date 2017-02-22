# AWS RDS Service Broker [![CircleCI](https://circleci.com/gh/AusDTO/pe-rds-broker.svg?style=svg)](https://circleci.com/gh/AusDTO/pe-rds-broker)

The RDS broker is a [cloud foundry service broker](https://docs.cloudfoundry.org/services/overview.html) originally based on
[cloudfoundry-community/pe-rds-broker](https://github.com/cloudfoundry-community/pe-rds-broker).
It is currently in the process of becoming production ready.
You can view the current state of the project on [waffle.io](https://waffle.io/AusDTO/pe-rds-broker).

***Disclaimer: While we are working to make the broker production ready, it is not ready yet. If you'd like to speed up the
process, feel free to [contribute](#contributing).***

The broker implements the [cloud foundry service broker API](https://docs.cloudfoundry.org/services/overview.html),
allowing developers to manage their own RDS instances for their cloud foundry applications.
It supports creating dedicated RDS instances with the following engines.

- [MySQL](https://aws.amazon.com/rds/mysql/)
- [PostgreSQL](https://aws.amazon.com/rds/postgresql/)
- [MariaDB](https://aws.amazon.com/rds/mariadb/)*
- [Aurora](https://aws.amazon.com/rds/aurora/)*

_* Support for these engines may be removed in the near future. If you particularly want us to keep them, let us know
by creating an issue._

It also supports creating databases on a shared RDS instance with the following engines.

- [MySQL](https://aws.amazon.com/rds/mysql/)
- [PostgreSQL](https://aws.amazon.com/rds/postgresql/)

The details of which databases can be created are managed by a configuration file. For more details about configuration,
see the [CONFIGURATION.md](CONFIGURATION.md).

## Managing instances

This section provides information for cloud foundry users who are managing their databases using this broker. If you
wish to deploy the broker to your cloud foundry instance or do development on the broker, see
[managing the broker](#managing-the-broker).

Once the broker is installed, you can manage your databases using the [cf cli](https://github.com/cloudfoundry/cli).
For a general introduction to managing cloud foundry services, see 
[the cloud foundry docs](https://docs.cloudfoundry.org/devguide/services/managing-services.html).

### Finding services and plans

The names of the services and plans and all their settings, are determined by the deployment configuration.
Run `cf marketplace` and/or `cf marketplace -s SERVICE` to find the details of the services and plans available to you.

### Dedicated or shared?

Before creating a database, you will need to decide which service and plan to use.
If both dedicated and shared instances are available you will need to choose one.

Dedicated instances run on their own RDS instance, with their own resource quotas, backups and restore points.
They are more expensive and slower to create and destroy but are recommended for production use.

All shared database instances for a particular engine (postgres or mysql) are on the same RDS instance. They cannot be
individually backed up or restored and if someone decides to use all the disk space, it will effect everyone. On the
other hand, they are cheaper and quick to create and destroy. They are recommended for development use.

### Multiple apps, one database

If you have multiple applications that need to bind to the same database (for instance, blue-green deploys), there are
some things to consider. By default, each application bound to a database gets a different username and password.
If you're using mysql, you can just bind all the apps to the database and it will work fine.
Postgres, on the other hand, does not support granting full read-write access to all tables in a database to an
arbitrary set of users. To get around this, you can bind all your applications to a postgres database with a custom
username. This will give all your applications the same access permissions on the data. You can do this by running something like

    cf bind-service APP_NAME SERVICE_INSTANCE -c '{"username":"awesome_user"}'

_Note: user bind parameters must be enabled in the deployment configuration for this to work._

### Database extensions

Many postgres database extensions require superuser access to enable them. The normal bind credentials are for an
unprivileged user so your applications cannot enable extensions themselves. To enable or disable extensions, run a
`cf update-service` command with the `extensions` parameter.

    cf update-service SERVICE_INSTANCE -c '{"extensions":["uuid-ossp","hstore"]}'

The broker will compare the provided list with the list of currently installed extensions and enable and disable
extensions as required.

If you would like to track your extensions in version control and update you database using your CI pipeline,
you can save the update parameters to a json file

```json
{
  "extensions": ["uuid-ossp", "hstore"]
}
```

and run

    cf update-service SERVICE_INSTANCE -c FILENAME

from your CI pipeline.

_Note: user update parameters must be enabled in the deployment configuration for this to work._

### All configuration options

This section details all the custom parameters used by the broker. For more details on specifying parameters, see
[managing services](https://docs.cloudfoundry.org/devguide/services/managing-services.html) in the cloud foundry docs
or run the specific `cf` command with `--help`.

#### Create parameters

If enabled by the deployment configuration, the broker supports the following parameters to the `cf create-service` command.

| Option                        | Type    | Description
|:------------------------------|:------- |:-----------
| backup_retention_period*      | integer | The number of days that Amazon RDS should retain automatic backups of the DB instance (between `0` and `35`)
| character_set_name*           | string  | For supported engines, indicates that the DB instance should be associated with the specified CharacterSet
| preferred_backup_window*      | string  | The daily time range during which automated backups are created if automated backups are enabled
| preferred_maintenance_window* | string  | The weekly time range during which system maintenance can occur

\* These parameters are ignored for shared instances.
Refer to the [Amazon Relational Database Service Documentation](https://aws.amazon.com/documentation/rds/)
for more details about how to set these properties.

### Update parameters

If enabled by the deployment configuration, the broker supports the following parameters to the `cf update-service` command.

| Option                       | Type      | Description
|:-----------------------------|:--------- |:-----------
| apply_immediately*            | boolean  | Specifies whether the modifications in this request and any pending modifications are asynchronously applied as soon as possible, regardless of the Preferred Maintenance Window setting for the DB instance
| backup_retention_period*      | integer  | The number of days that Amazon RDS should retain automatic backups of the DB instance (between `0` and `35`)
| preferred_backup_window*      | string   | The daily time range during which automated backups are created if automated backups are enabled
| preferred_maintenance_window* | string   | The weekly time range during which system maintenance can occur
| extensions^                   | []string | List of enabled database extensions

\* These parameters are ignored for shared instances.
Refer to the [Amazon Relational Database Service Documentation](https://aws.amazon.com/documentation/rds/)
for more details about how to set these properties.

^ Postgres only. `plpgsql` is always enabled and does not need to be included in this list.

#### Bind parameters

If enabled by the deployment configuration, the broker supports the following parameters to the `cf bind-service` command.

| Option   | Type   | Description
|:-------- |:------ |:-----------
| username | string | The username to use when connecting to the database (postgres only)

******************************************************

## Managing the broker

This section provides information for cloud foundry operators who wish to use the broker or do development on the
broker. If you just using the broker to manage your databases, see [managing instances](#managing-instances).

### Setup

Before running the broker, you will need to create a `config.yml` file, create any internal databases and set up your
environment variables.

#### config.yml

For more information on `config.yml`, see the [sample config file](config-sample.yml) and the 
[configuration docs](CONFIGURATION.md).

#### AWS credentials

The broker requires AWS credentials to manage RDS instances. [iam_policy.json](iam_policy.json) contains the 
[IAM](https://aws.amazon.com/iam/) permissions required by the broker. These credentials can be passed to the broker
in multiple ways, including via the environment variables `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`. For more
details on specifying the credentials, see the
[AWS SDK for Go documentation](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#id2).

#### Databases

There are up to three different databases required by the RDS broker. The internal database is used to store local
information and can use either sqlite3 or postgres. Obviously only postgres should be used in production but sqlite can
be useful during development. To create databases on a shared postgres or mysql instance, you will also need to set up 
the shared instance. There are a few scripts provided to simplify this process.

* [env.sample](env.sample) provides a minimal list of environment variables to get you up and going quickly in development.
  It uses sqlite3 for the internal database and does not provide any shared instances so you don't need to create
  any databases to get going.
* [bin/setup-dev-db.sh](bin/setup-dev-db.sh) creates a postgres internal database and uses that as the postgres shared
  instance. If mysql is installed, it will also create a mysql shared instance. It outputs a file called `db.env` with
  all the database environment variables the broker needs to connect to these databases. You should read the comments at the 
  beginning of that file to be sure your dev environment is set up to work with this script.
* [aws_db.tf](aws_db.tf) is a [terraform](https://www.terraform.io/) script to create a postgres internal database, 
  postgres shared instance and mysql shared instance on AWS. Read the documentation at the beginning of that file for
  more information on how to use it.

#### Other environment variables

There are a few other environment variables that need to be set for the broker to work.

Variable                 | Description
-------------------------|------------
RDSBROKER_USERNAME       | The username used by the cloud controller to authenticate to the broker
RDSBROKER_PASSWORD       | The password used by the cloud controller to authenticate to the broker
RDSBROKER_ENCRYPTION_KEY | The (hex-encoded) 256-bit key used to encrypt the passwords stored in the internal database

The username and password need to be the same as the ones passed to `cf create-service-broker`.
You can generate a random encryption key with something like `openssl rand -hex 32`.

### Installation

**WARNING: The installation section is from the original readme before the fork and may be out of date.**

#### Locally

Using the standard `go install` (you must have [Go](https://golang.org/) already installed in your local machine):

```
$ go install github.com/AusDTO/pe-rds-broker
$ pe-rds-broker -port=3000 -config=<path-to-your-config-file>
```

To pretty print the logs, pipe the output to [jq](https://stedolan.github.io/jq/). Note that this will remove any lines
that are not json.

```
$ pe-rds-broker -port=3000 -config=<path-to-your-config-file> | jq --unbuffered  -R 'fromjson?'
```

#### Cloud Foundry

The broker can be deployed to an already existing [Cloud Foundry](https://www.cloudfoundry.org/) installation:

```
$ git clone https://github.com/AusDTO/pe-rds-broker.git
$ cd pe-rds-broker
```

Modify the [included manifest file](manifest.yml) to include your AWS credentials and optionally the [sample configuration file](config-sample.yml). 
Then you can push the broker to your [Cloud Foundry](https://www.cloudfoundry.org/) environment:

```
$ cp config-sample.yml config.yml
$ cf push rds-broker
```

#### Docker

If you want to run the AWS RDS Service Broker on a Docker container, you can use the [cfplatformeng/rds-broker](https://registry.hub.docker.com/u/cfplatformeng/rds-broker/) Docker image.

```
$ docker run -d --name rds-broker -p 3000:3000 \
  -e AWS_ACCESS_KEY_ID=<your-aws-access-key-id> \
  -e AWS_SECRET_ACCESS_KEY=<your-aws-secret-access-key> \
  cfplatformeng/rds-broker
```

The Docker image comes with an [embedded sample configuration file](config-sample.yml). If you want to override it,
you can create the Docker image with you custom configuration file by running:

```
$ git clone https://github.com/AusDTO/pe-rds-broker.git
$ cd rds-broker
$ bin/build-docker-image
```

#### BOSH

This broker can be deployed using the [AWS Service Broker BOSH Release](https://github.com/cf-platform-eng/aws-broker-boshrelease).

### Managing the broker

Once the broker is configured and deployed, you will need to
[register the broker](https://docs.cloudfoundry.org/services/managing-service-brokers.html#register-broker) and
[make the services and plans public](https://docs.cloudfoundry.org/services/access-control.html#enable-access).


## Contributing

All contributions are welcome, large or small. Feel free to open an issue or pull request for whatever is bugging you.
If you're not sure about something, just open an issue with your question and (hopefully) someone will get back to you
soon. If you want to know what we're currently working on, look through the github issues or the kanban
representation of the issues on [waffle.io](https://waffle.io/AusDTO/pe-rds-broker).

## Copyright

Copyright (c) 2015 Pivotal Software Inc.

Copyright (c) 2017 Commonwealth of Australia

See [LICENSE](LICENSE) for details.
