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
	newExecuter := func(_ *Config, _ *rds.DBCluster, dbClusterEndpoint *rds.DBClusterEndpoint) executer {
		e := &mockExecuter{
			host:        *dbClusterEndpoint.Endpoint,
			expectedSQL: expectedSQL,
		}
		return e
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
				DBInstanceClass:      "db.t3.small",
				EnableExportTask:     true,
				ExportTaskIamRoleArn: "arn:aws:iam::000000000000:role/export-test",
				ExportTaskKmsKeyId:   "arn:aws:kms:ap-northeast-1:000000000000:key/00000000-0000-0000-0000-000000000000",
				ExportTaskS3Bucket:   "mascras-test-bucket",
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
				DBInstanceClass:      "db.t3.small",
				ExportTaskIdentifier: MockFailureExportTaskIdentifier,
				EnableExportTask:     true,
				ExportTaskIamRoleArn: "arn:aws:iam::000000000000:role/export-test",
				ExportTaskKmsKeyId:   "arn:aws:kms:ap-northeast-1:000000000000:key/00000000-0000-0000-0000-000000000000",
				ExportTaskS3Bucket:   "mascras-test-bucket",
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
			app.cfg.DBClusterIdentifier = c.clusterIdentifier
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
