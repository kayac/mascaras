# mascaras

this is backup tool for Aurora MySQL.  
mascaras creates Clone Aurora MySQL, execute the SQL, and then create a Snapshot.
## Usage

```shell
$ mascaras --help   
Usage: mascaras [options] <mask sql file> <source db cluster identifier>
         can use MASCARAS_ env prefix
  -database string
        Cloned Aurora DB sql target database.
  -db-cluster-identifier string
        Cloned Aurora DB Cluster Identifier
  -db-cluster-identifier-prefix string
        Cloned Aurora DB Cluster Identifier Prefix (default "mascaras-")
  -db-instance-class string
        Cloned Aurora DB Instance Class (default "db.t3.small")
  -db-user-name string
        Cloned Aurora DB user name (default "root")
  -db-user-password string
        Cloned Aurora DB user password.
  -debug
        enable debug log
  -enable-export-task
        created snapshot export to s3
  -export-task-export-only string
        export-task execute destination s3 key prefix
  -export-task-iam-role-arn string
        export-task execute IAM Role arn. required when enable export-task
  -export-task-identifier string
        export-task identifer.
  -export-task-kms-key-id string
        export-task KMS Key ID. required when enable export-task
  -export-task-s3-bucket string
        export-task destination s3 bucket name. required when enable export-task
  -export-task-s3-prefix string
        export-task execute destination s3 key prefix
  -help
        show help
  -publicly-accessible
        Cloned Aurora DB PubliclyAccessible.
  -security-group-ids string
        Cloned Aurora DB Cluster Secturity Group IDs
  -version
        show version
```

mascaras Reads environment variables with the MASCARAS_ prefix.
MASCARAS_DB_CLUSTER_IDENTIFIER_PREFIX is read as db-cluster-identifier-prefix.

### For example 

Consider the case of backing up an Aurora MySQL cluster with the identifier database-src.
sppose want to mask this cluster under the following conditions.

- db user name: user01
- db user password: hoge1234
- target database: db01
- sql filename: mask.sql

the DB schema and the contents of mask.sql look like this:

- db schema:
```sql
CREATE TABLE db01.users (
    id int auto_increment,
    name varchar(191),
    PRIMARY KEY (`id`),
    UNIQUE INDEX `name` (`name`)
);
```

- mask.sql
```sql
BEGIN;

update users
set name = md5(name);

COMMIT;
```

mascaras works as follows.  

```shell
$ export MASCARAS_DB_USER_PASSWORD=hoge1234
$ mascaras --db-user-name user01 -database db01 ./mask.sql database-src
2021/06/10 16:46:23 [info] cloned db cluster: arn:aws:rds:ap-northeast-1:012345678900:cluster:mascaras-nrqmae42fl
2021/06/10 16:46:23 [info] wait db cluster `mascaras-nRqMaE42fL` status available...
2021/06/10 16:46:23 [info] now db cluster status is creating ...
2021/06/10 16:47:23 [info] now db cluster status is creating ...
2021/06/10 16:48:24 [info] db cluster status is available!
2021/06/10 16:48:25 [info] create db instance: arn:aws:rds:ap-northeast-1:012345678900:db:mascaras-nrqmae42fl-instance
2021/06/10 16:48:25 [info] wait db instance `mascaras-nRqMaE42fL-instance` status available...
2021/06/10 16:48:25 [info] now db instance status is creating ...
2021/06/10 16:49:24 [info] now db instance status is creating ...
2021/06/10 16:50:22 [info] now db instance status is creating ...
2021/06/10 16:51:24 [info] now db instance status is creating ...
2021/06/10 16:52:24 [info] now db instance status is creating ...
2021/06/10 16:53:25 [info] db instance status is available!
2021/06/10 16:53:25 [info] wait db endpoints `mascaras-nRqMaE42fL` status available...
2021/06/10 16:53:25 [info] db cluster endpoint status is available!
2021/06/10 16:53:25 [info] start do sql `testdata/mask.sql`
2021/06/10 16:53:25 [info] end do sql
2021/06/10 16:53:25 [info] wait db cluster `mascaras-nRqMaE42fL` LatestRestorableTime past masked time `2021-06-10T07:53:25Z`...
2021/06/10 16:53:25 [info] now db cluster LatestRestorableTime=2021-06-10T07:47:38Z
2021/06/10 16:54:24 [info] now db cluster LatestRestorableTime=2021-06-10T07:47:38Z
2021/06/10 16:55:35 [info] now db cluster LatestRestorableTime=2021-06-10T07:47:38Z
2021/06/10 16:56:35 [info] now db cluster LatestRestorableTime=2021-06-10T07:47:38Z
2021/06/10 16:57:26 [info] now db cluster LatestRestorableTime=2021-06-10T07:47:38Z
2021/06/10 16:58:25 [info] now db cluster LatestRestorableTime=2021-06-10T07:47:38Z
2021/06/10 16:59:27 [info] now db cluster LatestRestorableTime=2021-06-10T07:47:38Z
2021/06/10 17:00:55 [info] db cluster LatestRestorableTime=2021-06-10T07:58:21Z, complete!
2021/06/10 17:00:55 [info] start cleanup ...
2021/06/10 17:00:56 [info] delete temp db instance:arn:aws:rds:ap-northeast-1:012345678900:db:mascaras-nrqmae42fl-instance
2021/06/10 17:00:56 [info] delete temp db cluster:arn:aws:rds:ap-northeast-1:012345678900:cluster:mascaras-nrqmae42fl
2021/06/10 17:00:56 [info] finish cleanup
2021/06/10 17:01:01 [info] success
```

# LICENCE

MIT

# Author

KAYAC Inc.
