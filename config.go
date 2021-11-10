package mascaras

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"text/template"

	"github.com/Songmu/flextime"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	gconf "github.com/kayac/go-config"
)

type Config struct {
	TempCluster               TempDBClusterConfig `json:"temp_cluster,omitempty" yaml:"temp_cluster,omitempty"`
	DBUserName                string              `json:"db_user_name,omitempty" yaml:"db_user_name,omitempty"`
	DBUserPassword            string              `json:"db_user_password,omitempty" yaml:"db_user_password,omitempty"`
	Database                  string              `json:"database,omitempty" yaml:"database,omitempty"`
	SSLMode                   string              `json:"ssl_mode,omitempty" yaml:"ssl_mode,omitempty"`
	SQLFile                   string              `json:"sql_file,omitempty" yaml:"sql_file,omitempty"`
	SourceDBClusterIdentifier string              `json:"source_db_cluster_identifier,omitempty" yaml:"source_db_cluster_identifier,omitempty"`
	Interactive               bool                `json:"interactive,omitempty" yaml:"interactive,omitempty"`

	EnableExportTask bool             `json:"enable_export_task,omitempty" yaml:"enable_export_task,omitempty"`
	ExportTask       ExportTaskConfig `json:"export_task,omitempty" yaml:"export_task,omitempty"`
}

type TempDBClusterConfig struct {
	DBClusterIdentifierPrefix string `json:"db_cluster_identifier_prefix,omitempty" yaml:"db_cluster_identifier_prefix,omitempty"`
	DBClusterIdentifier       string `json:"db_cluster_identifier,omitempty" yaml:"db_cluster_identifier,omitempty"`
	DBInstanceClass           string `json:"db_instance_class,omitempty" yaml:"db_instance_class,omitempty"`
	SecurityGroupIDs          string `json:"security_group_ids,omitempty" yaml:"security_group_ids,omitempty"`
	PubliclyAccessible        bool   `json:"publicly_accessible,omitempty" yaml:"publicly_accessible,omitempty"`
}

type ExportTaskConfig struct {
	TaskIdentifier string `json:"task_identifier,omitempty" yaml:"task_identifier,omitempty"`
	IAMRoleArn     string `json:"iam_role_arn,omitempty" yaml:"iam_role_arn,omitempty"`
	KMSKeyId       string `json:"kms_key_id,omitempty" yaml:"kms_key_id,omitempty"`
	S3Bucket       string `json:"s3_bucket,omitempty" yaml:"s3_bucket,omitempty"`
	S3Prefix       string `json:"s3_prefix,omitempty" yaml:"s3_prefix,omitempty"`
	ExportOnly     string `json:"export_only,omitempty" yaml:"export_only,omitempty"`
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
		SSLMode:          "disable",
	}
}

func LoadConfig(loc string) (*Config, error) {
	r, err := openLocation(loc)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	bs, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	gconf.Funcs(template.FuncMap{
		"now": flextime.Now,
	})
	return cfg, gconf.LoadWithEnvBytes(&cfg, bs)
}

func (cfg *Config) SetFlags(f *flag.FlagSet) {
	cfg.TempCluster.SetFlags(f)
	f.StringVar(&cfg.DBUserName, "db-user-name", cfg.DBUserName, "Cloned Aurora DB user name")
	f.StringVar(&cfg.DBUserPassword, "db-user-password", cfg.DBUserPassword, "Cloned Aurora DB user password.")
	f.StringVar(&cfg.Database, "database", cfg.Database, "Cloned Aurora DB sql target database.")
	f.BoolVar(&cfg.EnableExportTask, "enable-export-task", cfg.EnableExportTask, "created snapshot export to s3")
	f.StringVar(&cfg.SSLMode, "ssl-mode", cfg.SSLMode, "ssl mode setting apply only PostgreSQL type Aurora DB")
	f.StringVar(&cfg.SQLFile, "sql-file", cfg.SQLFile, "")
	f.StringVar(&cfg.SourceDBClusterIdentifier, "src-db-cluster", cfg.SourceDBClusterIdentifier, "")
	f.BoolVar(&cfg.Interactive, "interactive", cfg.Interactive, "after mask sql,　Launch an interactive prompt after executing SQL")
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

func coalesceString(str1, str2 string) string {
	if str1 == "" {
		return str2
	}
	return str1
}

func (cfg *Config) MergeIn(o *Config) *Config {
	cfg.TempCluster.MergIn(&o.TempCluster)
	cfg.DBUserName = coalesceString(o.DBUserName, cfg.DBUserName)
	cfg.DBUserPassword = coalesceString(o.DBUserPassword, cfg.DBUserPassword)
	cfg.Database = coalesceString(o.Database, cfg.Database)
	cfg.EnableExportTask = o.EnableExportTask || cfg.EnableExportTask
	cfg.SSLMode = coalesceString(o.SSLMode, cfg.SSLMode)
	cfg.SQLFile = coalesceString(o.SQLFile, cfg.SQLFile)
	cfg.SourceDBClusterIdentifier = coalesceString(o.SourceDBClusterIdentifier, cfg.SourceDBClusterIdentifier)
	cfg.Interactive = o.Interactive || cfg.Interactive
	cfg.ExportTask.MergIn(&o.ExportTask)
	return cfg
}

func (cfg *TempDBClusterConfig) MergIn(o *TempDBClusterConfig) *TempDBClusterConfig {
	cfg.DBClusterIdentifier = coalesceString(o.DBClusterIdentifier, cfg.DBClusterIdentifier)
	cfg.DBClusterIdentifierPrefix = coalesceString(o.DBClusterIdentifierPrefix, cfg.DBClusterIdentifierPrefix)
	cfg.DBInstanceClass = coalesceString(o.DBInstanceClass, cfg.DBInstanceClass)
	cfg.SecurityGroupIDs = coalesceString(o.SecurityGroupIDs, cfg.SecurityGroupIDs)
	cfg.PubliclyAccessible = o.PubliclyAccessible || cfg.PubliclyAccessible
	return cfg
}

func (cfg *ExportTaskConfig) MergIn(o *ExportTaskConfig) *ExportTaskConfig {
	cfg.TaskIdentifier = coalesceString(o.TaskIdentifier, cfg.TaskIdentifier)
	cfg.IAMRoleArn = coalesceString(o.IAMRoleArn, cfg.IAMRoleArn)
	cfg.KMSKeyId = coalesceString(o.KMSKeyId, cfg.KMSKeyId)
	cfg.S3Bucket = coalesceString(o.S3Bucket, cfg.S3Bucket)
	cfg.S3Prefix = coalesceString(o.S3Prefix, cfg.S3Prefix)
	cfg.ExportOnly = coalesceString(o.ExportOnly, cfg.ExportOnly)
	return cfg
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

func openLocation(loc string) (io.ReadCloser, error) {
	if u, err := url.Parse(loc); err == nil {
		if u.Scheme == "" {
			return os.Open(loc)
		}
		if u.Scheme == "file" {
			return os.Open(u.Path)
		}
		if u.Scheme == "s3" {
			log.Println("[debug] get from s3 loc=", loc)
			return openS3(u)
		}
		return nil, fmt.Errorf("schema %s is not support, can not get %s", u.Scheme, loc)
	}
	return os.Open(loc)
}

func openS3(u *url.URL) (io.ReadCloser, error) {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		log.Println("[debug] missing region")
		var err error
		region, err = s3manager.GetBucketRegion(
			context.Background(),
			session.Must(session.NewSession()),
			u.Host,
			"us-east-1",
		)
		if err != nil {
			return nil, err
		}
	}
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, err
	}
	svc := s3.New(sess)
	log.Printf("[debug] try get bucket=%s key=%s\n", u.Host, u.Path)
	result, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(u.Host),
		Key:    aws.String(u.Path),
	})
	if err != nil {
		return nil, err
	}
	return result.Body, err
}
