package mascaras

import (
	"errors"
	"flag"
	"log"
	"strings"
)

type Config struct {
	DBClusterIdentifierPrefix string
	DBClusterIdentifier       string
	DBInstanceClass           string
	DBUserName                string
	DBUserPassword            string
	Database                  string
	PubliclyAccessible        bool
	securityGroupIDs          string

	EnableExportTask     bool
	ExportTaskIdentifier string
	ExportTaskIamRoleArn string
	ExportTaskKmsKeyId   string
	ExportTaskS3Bucket   string
	ExportTaskS3Prefix   string
	exportTaskExportOnly string
}

func DefaultConfig() *Config {
	return &Config{
		DBClusterIdentifierPrefix: "mascaras-",
		DBInstanceClass:           "db.t3.small",
		DBUserName:                "root",
		PubliclyAccessible:        false,
		EnableExportTask:          false,
	}
}

func (cfg *Config) SetFlags(f *flag.FlagSet) {
	f.StringVar(&cfg.DBClusterIdentifierPrefix, "db-cluster-identifier-prefix", cfg.DBClusterIdentifierPrefix, "Cloned Aurora DB Cluster Identifier Prefix")
	f.StringVar(&cfg.DBClusterIdentifier, "db-cluster-identifier", cfg.DBClusterIdentifier, "Cloned Aurora DB Cluster Identifier")
	f.StringVar(&cfg.DBInstanceClass, "db-instance-class", cfg.DBInstanceClass, "Cloned Aurora DB Instance Class")
	f.StringVar(&cfg.DBUserName, "db-user-name", cfg.DBUserName, "Cloned Aurora DB user name")
	f.StringVar(&cfg.DBUserPassword, "db-user-password", cfg.DBUserPassword, "Cloned Aurora DB user password.")
	f.StringVar(&cfg.Database, "database", cfg.Database, "Cloned Aurora DB sql target database.")
	f.BoolVar(&cfg.PubliclyAccessible, "publicly-accessible", cfg.PubliclyAccessible, "Cloned Aurora DB PubliclyAccessible.")
	f.StringVar(&cfg.securityGroupIDs, "security-group-ids", cfg.securityGroupIDs, "Cloned Aurora DB Cluster Secturity Group IDs")
	f.BoolVar(&cfg.EnableExportTask, "enable-export-task", cfg.EnableExportTask, "created snapshot export to s3")
	f.StringVar(&cfg.ExportTaskIdentifier, "export-task-identifier", cfg.ExportTaskIdentifier, "export-task identifer.")
	f.StringVar(&cfg.ExportTaskIamRoleArn, "export-task-iam-role-arn", cfg.ExportTaskIamRoleArn, "export-task execute IAM Role arn. required when enable export-task")
	f.StringVar(&cfg.ExportTaskKmsKeyId, "export-task-kms-key-id", cfg.ExportTaskKmsKeyId, "export-task KMS Key ID. required when enable export-task")
	f.StringVar(&cfg.ExportTaskS3Bucket, "export-task-s3-bucket", cfg.ExportTaskS3Bucket, "export-task destination s3 bucket name. required when enable export-task")
	f.StringVar(&cfg.ExportTaskS3Prefix, "export-task-s3-prefix", cfg.ExportTaskS3Prefix, "export-task execute destination s3 key prefix")
	f.StringVar(&cfg.exportTaskExportOnly, "export-task-export-only", cfg.exportTaskExportOnly, "export-task execute destination s3 key prefix")
}

func (cfg *Config) Validate() error {
	if cfg.DBClusterIdentifier == "" {
		if cfg.DBClusterIdentifierPrefix == "" {
			return errors.New("either db-cluster-identifier or db-cluster-identifier-prefix is required")
		}
	}

	if cfg.DBInstanceClass == "" {
		return errors.New("db-instance-class is required")
	}
	if !strings.HasPrefix(cfg.DBInstanceClass, "db.") {
		log.Println("[warn] db-instance-class does not have the `db.` prefix. Maybe you can't create a DB instance")
	}
	if cfg.DBUserName == "" {
		log.Println("[warn] db-user-name is empty. maybe can not connect Cloaned Aurora")
	}
	if cfg.DBUserPassword == "" {
		log.Println("[warn] db-user-password is empty. maybe can not connect Cloaned Aurora")
	}

	if !cfg.EnableExportTask {
		return nil
	}
	//In case Enable ExportTask
	if cfg.ExportTaskIamRoleArn == "" {
		return errors.New("export-task-iam-role-arn is required if ExportTask is enabled")
	}
	if cfg.ExportTaskKmsKeyId == "" {
		return errors.New("export-task-kms-key-id is required if ExportTask is enabled")
	}
	if cfg.ExportTaskS3Bucket == "" {
		return errors.New("export-task-s3-bucket is required if ExportTask is enabled")
	}
	return nil
}

func (cfg *Config) SecurityGroupIDs() []string {
	if cfg.securityGroupIDs == "" {
		return nil
	}
	return strings.Split(cfg.securityGroupIDs, ",")
}

func (cfg *Config) ExportTaskExportOnly() []string {
	if cfg.exportTaskExportOnly == "" {
		return nil
	}
	return strings.Split(cfg.exportTaskExportOnly, ",")
}
