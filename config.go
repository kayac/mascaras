package mascaras

import (
	"errors"
	"flag"
	"log"
	"strings"
)

type Config struct {
	TempCluster    TempDBClusterConfig
	DBUserName     string
	DBUserPassword string
	Database       string

	EnableExportTask bool
	ExportTask       ExportTaskConfig
}

type TempDBClusterConfig struct {
	DBClusterIdentifierPrefix string
	DBClusterIdentifier       string
	DBInstanceClass           string
	SecurityGroupIDs          string
	PubliclyAccessible        bool
}

type ExportTaskConfig struct {
	TaskIdentifier string
	IAMRoleArn     string
	KMSKeyId       string
	S3Bucket       string
	S3Prefix       string
	ExportOnly     string
}

func DefaultConfig() *Config {
	return &Config{
		TempCluster: TempDBClusterConfig{
			DBClusterIdentifierPrefix: "mascaras-",
			DBInstanceClass:           "db.t3.small",
			PubliclyAccessible:        false,
		},
		DBUserName:       "root",
		EnableExportTask: false,
	}
}

func (cfg *Config) SetFlags(f *flag.FlagSet) {
	cfg.TempCluster.SetFlags(f)
	f.StringVar(&cfg.DBUserName, "db-user-name", cfg.DBUserName, "Cloned Aurora DB user name")
	f.StringVar(&cfg.DBUserPassword, "db-user-password", cfg.DBUserPassword, "Cloned Aurora DB user password.")
	f.StringVar(&cfg.Database, "database", cfg.Database, "Cloned Aurora DB sql target database.")
	f.BoolVar(&cfg.EnableExportTask, "enable-export-task", cfg.EnableExportTask, "created snapshot export to s3")
	cfg.ExportTask.SetFlags(f)
}

func (cfg *TempDBClusterConfig) SetFlags(f *flag.FlagSet) {
	f.StringVar(&cfg.DBClusterIdentifierPrefix, "db-cluster-identifier-prefix", cfg.DBClusterIdentifierPrefix, "Cloned Aurora DB Cluster Identifier Prefix")
	f.StringVar(&cfg.DBClusterIdentifier, "db-cluster-identifier", cfg.DBClusterIdentifier, "Cloned Aurora DB Cluster Identifier")
	f.StringVar(&cfg.DBInstanceClass, "db-instance-class", cfg.DBInstanceClass, "Cloned Aurora DB Instance Class")
	f.BoolVar(&cfg.PubliclyAccessible, "publicly-accessible", cfg.PubliclyAccessible, "Cloned Aurora DB PubliclyAccessible.")
	f.StringVar(&cfg.SecurityGroupIDs, "security-group-ids", cfg.SecurityGroupIDs, "Cloned Aurora DB Cluster Secturity Group IDs")
}

func (cfg *ExportTaskConfig) SetFlags(f *flag.FlagSet) {
	f.StringVar(&cfg.TaskIdentifier, "export-task-identifier", cfg.TaskIdentifier, "export-task identifer.")
	f.StringVar(&cfg.IAMRoleArn, "export-task-iam-role-arn", cfg.IAMRoleArn, "export-task execute IAM Role arn. required when enable export-task")
	f.StringVar(&cfg.KMSKeyId, "export-task-kms-key-id", cfg.KMSKeyId, "export-task KMS Key ID. required when enable export-task")
	f.StringVar(&cfg.S3Bucket, "export-task-s3-bucket", cfg.S3Bucket, "export-task destination s3 bucket name. required when enable export-task")
	f.StringVar(&cfg.S3Prefix, "export-task-s3-prefix", cfg.S3Prefix, "export-task execute destination s3 key prefix")
	f.StringVar(&cfg.ExportOnly, "export-task-export-only", cfg.ExportOnly, "export-task execute destination s3 key prefix")

}

func (cfg *Config) Validate() error {
	if err := cfg.TempCluster.Validate(); err != nil {
		return nil
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
	return cfg.ExportTask.Validate()
}

func (cfg *TempDBClusterConfig) Validate() error {
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
	return nil
}

func (cfg *ExportTaskConfig) Validate() error {
	//In case Enable ExportTask
	if cfg.IAMRoleArn == "" {
		return errors.New("export-task-iam-role-arn is required if ExportTask is enabled")
	}
	if cfg.KMSKeyId == "" {
		return errors.New("export-task-kms-key-id is required if ExportTask is enabled")
	}
	if cfg.S3Bucket == "" {
		return errors.New("export-task-s3-bucket is required if ExportTask is enabled")
	}
	return nil
}

func (cfg *TempDBClusterConfig) securityGroupIDs() []string {
	if cfg.SecurityGroupIDs == "" {
		return nil
	}
	return strings.Split(cfg.SecurityGroupIDs, ",")
}

func (cfg *ExportTaskConfig) exportOnly() []string {
	if cfg.ExportOnly == "" {
		return nil
	}
	return strings.Split(cfg.ExportOnly, ",")
}
