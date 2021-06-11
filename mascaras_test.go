package mascaras

import (
	"bytes"
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/stretchr/testify/require"
)

func TestAppRun(t *testing.T) {
	expectedSQL, err := readSQL("testdata/mask.sql")
	require.NoError(t, err)
	newExecuter := func(_ *Config, _ *rds.DBCluster, dbClusterEndpoint *rds.DBClusterEndpoint) (executer, error) {
		e := &mockExecuter{
			host:        *dbClusterEndpoint.Endpoint,
			expectedSQL: expectedSQL,
		}
		return e, nil
	}
	cases := []struct {
		cfg               *Config
		clusterIdentifier string
		errMsg            string
	}{
		{
			clusterIdentifier: MockSuccessDBClusterIdentifier,
		},
		{
			clusterIdentifier: MockSuccessDBClusterIdentifier,
			cfg: &Config{
				TempCluster: TempDBClusterConfig{
					DBInstanceClass: "db.t3.small",
				},
				EnableExportTask: true,
				ExportTask: ExportTaskConfig{
					IAMRoleArn: "arn:aws:iam::000000000000:role/export-test",
					KMSKeyId:   "arn:aws:kms:ap-northeast-1:000000000000:key/00000000-0000-0000-0000-000000000000",
					S3Bucket:   "mascras-test-bucket",
				},
			},
		},
		{
			clusterIdentifier: MockFailureRestoreDBClusterIdentifier,
			errMsg:            "RestoreDBClusterToPointInTime:failure RestoreDBClusterToPointInTimeWithContext",
		},
		{
			clusterIdentifier: MockFailureCreateInstanceDBClusterIdentifier,
			errMsg:            "failure CreateDBInstanceWithContext",
		},
		{
			clusterIdentifier: MockFailureExecuteSQLDBClusterIdentifier,
			errMsg:            "failed ExecuteContext",
		},
		{
			clusterIdentifier: MockFailureCreateSnapshotDBClusterIdentifier,
			errMsg:            "failure CreateDBClusterSnapshotWithContext",
		},
		{
			clusterIdentifier: MockSuccessDBClusterIdentifier,
			cfg: &Config{
				TempCluster: TempDBClusterConfig{
					DBInstanceClass: "db.t3.small",
				},
				EnableExportTask: true,
				ExportTask: ExportTaskConfig{
					TaskIdentifier: MockFailureExportTaskIdentifier,
					IAMRoleArn:     "arn:aws:iam::000000000000:role/export-test",
					KMSKeyId:       "arn:aws:kms:ap-northeast-1:000000000000:key/00000000-0000-0000-0000-000000000000",
					S3Bucket:       "mascras-test-bucket",
				},
			},
			errMsg: "failure StartExportTaskWithContext",
		},
	}
	for _, c := range cases {
		t.Run(c.clusterIdentifier, func(t *testing.T) {
			cleanup := setLogOutput(t)
			defer cleanup()
			svc := &mockRDSService{}
			app := &App{
				rdsSvc:       svc,
				baseInterval: time.Millisecond,
				newExecuter:  newExecuter,
			}
			if c.cfg == nil {
				app.cfg = DefaultConfig()
			} else {
				app.cfg = c.cfg
			}
			app.cfg.TempCluster.DBClusterIdentifier = c.clusterIdentifier
			require.NoError(t, app.cfg.Validate(), "config validate no error")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := app.Run(ctx, "testdata/mask.sql", "mascaras-test")
			if c.errMsg == "" {
				require.NoError(t, err, "run no error")
			} else {
				require.EqualError(t, err, c.errMsg, "run expected error")
			}
			if svc.isCreateCluster {
				require.True(t, svc.isDeleteCluster)
			}
			if svc.isCreateInstance {
				require.True(t, svc.isDeleteInstance)
			}
		})
	}
}

func setLogOutput(t *testing.T) func() {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	return func() {
		t.Log(buf.String())
		log.SetOutput(os.Stderr)
	}
}

func TestConfigMergeIn(t *testing.T) {
	os.Setenv("PASSWORD", "super_password")
	o, err := LoadConfig("testdata/config.yml")
	require.NoError(t, err)
	cfg := &Config{
		TempCluster: TempDBClusterConfig{
			DBInstanceClass: "db.r5.xlarge",
		},
		Database:         "db01",
		EnableExportTask: true,
		ExportTask: ExportTaskConfig{
			IAMRoleArn: "arn:aws:iam::000000000000:role/export-test",
			KMSKeyId:   "arn:aws:kms:ap-northeast-1:000000000000:key/00000000-0000-0000-0000-000000000000",
			S3Bucket:   "mascras-test-bucket",
		},
	}
	cfg = o.MergeIn(cfg)
	expected := &Config{
		TempCluster: TempDBClusterConfig{
			DBClusterIdentifierPrefix: "mascaras-",
			DBClusterIdentifier:       "test",
			DBInstanceClass:           "db.r5.xlarge",
			SecurityGroupIDs:          "sg-12345,sg-354321",
			PubliclyAccessible:        true,
		},
		DBUserName:       "admin",
		DBUserPassword:   "super_password",
		Database:         "db01",
		EnableExportTask: true,
		ExportTask: ExportTaskConfig{
			TaskIdentifier: "test-out",
			IAMRoleArn:     "arn:aws:iam::000000000000:role/export-test",
			KMSKeyId:       "arn:aws:kms:ap-northeast-1:000000000000:key/00000000-0000-0000-0000-000000000000",
			S3Bucket:       "mascras-test-bucket",
		},
	}
	require.EqualValues(t, expected, cfg)
}
