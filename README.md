# mascaras

this is backup tool for Aurora MySQL.  
mascaras creates Clone Aurora MySQL, execute the SQL, and then create a Snapshot.
## Usage

```shell
$ mascaras --help
Usage: mascaras [options] <source db cluster identifier>
         can use MASCARAS_ env prefix
  -config string
        config file path
  -database string
        Cloned Aurora DB sql target database.
  -db-cluster-identifier string
        Cloned Aurora DB Cluster Identifier
  -db-cluster-identifier-prefix string
        Cloned Aurora DB Cluster Identifier Prefix
  -db-instance-class string
        Cloned Aurora DB Instance Class
  -db-user-name string
        Cloned Aurora DB user name
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
  -interactive
        after mask sql,　Launch an interactive prompt after executing SQL
  -publicly-accessible
        Cloned Aurora DB PubliclyAccessible.
  -security-group-ids string
        Cloned Aurora DB Cluster Secturity Group IDs
  -sql-file string
    
  -src-db-cluster string
    
  -version
        show version
```

mascaras Reads environment variables with the MASCARAS_ prefix.  
MASCARAS_DB_CLUSTER_IDENTIFIER_PREFIX is read as db-cluster-identifier-prefix.

You can also use a configuration file. The format is as follows.

```yaml
temp_cluster:
  db_cluster_identifier_prefix: mascaras
  db_instance_class: db.t3.small
  security_group_ids: sg-0000001,sg-000002
  publicly_accessible: true

db_user_name: user01
db_user_password: {{ must_env `DB_PASSWORD` }}
database: mascaras
sql_file: s3://mascaras-data/mask.sql
source_db_cluster_identifier: mascaras-src

enable_export_task: false
export_task:
  iam_role_arn: arn:aws:iam::000000000000:role/export-role
  kms_key_id: arn:aws:kms:ap-northeast-1:000000000000:key/00000000-0000-0000-0000-000000000000
  s3_bucket: snapshot-export-target
  s3_prefix: db01/export
  export_only: mascaras.users,mascaras.roles
```
See [github.com/kayac/go-config](https://github.com/kayac/go-config) for template syntax.  


The priority of the settings is as follows.
```
[console flag and args] > [environment variable] > [config file]  
```

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
$ mascaras -db-user-name user01 -database db01 -sql-file ./mask.sql database-src
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
2021/06/10 16:53:25 [info] start do sql `./mask.sql`
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

## Extended usage (intaractive)

`-interactive` will launch a simple Prompt after running sql.
For example, if you want to check the temporary mask data, you can use it as shown in the example below.

```shell
$ mascaras --config /path/to/config --interactive
2021/06/11 14:42:48 [info] cloned db cluster: arn:aws:rds:ap-northeast-1:012345678900:cluster:mascaras-test-cojruk7qan
2021/06/11 14:42:49 [info] create db instance: arn:aws:rds:ap-northeast-1:012345678900:db:mascaras-test-cojruk7qan-instance
2021/06/11 14:42:49 [info] wait db cluster `mascaras-test-coJRUK7QAn` status available...
2021/06/11 14:42:49 [info] now db cluster status is creating ...
2021/06/11 14:43:52 [info] now db cluster status is creating ...
2021/06/11 14:44:52 [info] db cluster status is available!
2021/06/11 14:44:52 [info] wait db instance `mascaras-test-coJRUK7QAn-instance` status available...
2021/06/11 14:44:52 [info] now db instance status is creating ...
2021/06/11 14:45:49 [info] now db instance status is creating ...
2021/06/11 14:46:47 [info] now db instance status is creating ...
2021/06/11 14:47:46 [info] now db instance status is creating ...
2021/06/11 14:48:49 [info] db instance status is available!
2021/06/11 14:48:49 [info] wait db endpoints `mascaras-test-coJRUK7QAn` status available...
2021/06/11 14:48:49 [info] db cluster endpoint status is available!
2021/06/11 14:48:49 [info] start do sql `./mask.sql`
2021/06/11 14:48:49 [info] end do sql
2021/06/11 14:48:49 [info] start interactive
2021/06/11 14:48:49 [info] 
2021/06/11 14:48:49 [info] Use the `exit` or` abort` command to escape from Prompt.
2021/06/11 14:48:49 [info] Enter `help` command for more information.
2021/06/11 14:48:49 [info] Note: `^C` behaves the same as the `abort` command.
aurora[mascaras-test-cojruk7qan]>show databases;

+--------------------+
|      DATABASE      |
+--------------------+
| information_schema |
| mascaras           |
| mysql              |
| performance_schema |
| sys                |
+--------------------+

aurora[mascaras-test-cojruk7qan]>show tables;

+--------------------+
| TABLES IN MASCARAS |
+--------------------+
| users              |
+--------------------+

aurora[mascaras-test-cojruk7qan]>SELECT * FROM users LIMIT 5;

+-----+----------------------------------+
| ID  |               NAME               |
+-----+----------------------------------+
| 143 | 06f37d46903da0688fb3722daa94e1c4 |
| 167 | 08d8cef37c04ffa21251f3d23c0cfada |
| 149 | 099ba4351b5ce4d24f86c9ff7975a768 |
| 129 | 0c9056e25e586a8d1c68656a8707dae3 |
| 166 | 0d6ca2221b886bea2e08c7cd9c996480 |
+-----+----------------------------------+

aurora[mascaras-test-cojruk7qan]>show create table users;

+-------+--------------------------------+
| TABLE |          CREATE TABLE          |
+-------+--------------------------------+
| users | CREATE TABLE `users` (         |
|       |   `id` int(11) NOT NULL        |
|       | AUTO_INCREMENT,   `name`       |
|       | varchar(191) DEFAULT NULL,     |
|       |   PRIMARY KEY (`id`),          |
|       |   UNIQUE KEY `name`            |
|       | (`name`) ) ENGINE=InnoDB       |
|       | AUTO_INCREMENT=186 DEFAULT     |
|       | CHARSET=latin1                 |
+-------+--------------------------------+

aurora[mascaras-test-cojruk7qan]>select * form users;
query rows failed: Error 1064: You have an error in your SQL syntax; check the manual that corresponds to your MySQL server version for the right syntax to use near 'form users' at line 1

aurora[mascaras-test-cojruk7qan]>help
commands:
        abort:  Exit prompt as abnormal. Does not create a snapshot
        exit:   Exit prompt as successful, continue creating Snapshot

aurora[mascaras-test-cojruk7qan]>exit
exit prompt.
2021/06/11 14:56:23 [info] end interactive
2021/06/11 14:56:23 [info] wait db cluster `mascaras-test-coJRUK7QAn` LatestRestorableTime past masked time `2021-06-11T05:56:11Z`...
2021/06/11 14:56:23 [info] now db cluster LatestRestorableTime=2021-06-11T05:53:35Z
2021/06/11 14:57:21 [info] now db cluster LatestRestorableTime=2021-06-11T05:53:35Z
2021/06/11 14:58:22 [info] now db cluster LatestRestorableTime=2021-06-11T05:53:35Z
2021/06/11 14:59:20 [info] now db cluster LatestRestorableTime=2021-06-11T05:53:35Z
2021/06/11 15:00:19 [info] db cluster LatestRestorableTime=2021-06-11T05:57:39Z, complete!
2021/06/11 15:00:19 [info] create snapshot: mascaras-test-coJRUK7QAn-snapshot
2021/06/11 15:00:19 [info] success arn = arn:aws:rds:ap-northeast-1:012345678900:cluster-snapshot:mascaras-test-cojruk7qan-snapshot
2021/06/11 15:00:19 [info] start cleanup ...
2021/06/11 15:00:19 [info] delete temp db instance:arn:aws:rds:ap-northeast-1:012345678900:db:mascaras-test-cojruk7qan-instance
2021/06/11 15:00:19 [info] delete temp db cluster:arn:aws:rds:ap-northeast-1:012345678900:cluster:mascaras-test-cojruk7qan
2021/06/11 15:00:19 [info] finish cleanup
2021/06/11 15:00:19 [info] success.
```

# LICENCE

MIT

# Author

KAYAC Inc.
