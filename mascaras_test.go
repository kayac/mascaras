package mascaras

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Songmu/flextime"
	"github.com/stretchr/testify/require"
)

func TestAppRun(t *testing.T) {
	expectedSQLbase, err := readSQL("testdata/mask.sql")
	require.NoError(t, err)
	cases := []struct {
		casetag           string
		cfg               *Config
		clusterIdentifier string
		errMsg            string
		expectedSQL       string
		noMask            bool
		stdin             string
	}{
		{
			clusterIdentifier: MockSuccessDBClusterIdentifier,
			expectedSQL:       expectedSQLbase,
		},
		{
			casetag:           "export task success",
			clusterIdentifier: MockSuccessDBClusterIdentifier,
			expectedSQL:       expectedSQLbase,
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
			expectedSQL:       expectedSQLbase,
		},
		{
			casetag:           "export task failed",
			clusterIdentifier: MockSuccessDBClusterIdentifier,
			expectedSQL:       expectedSQLbase,
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
		{
			casetag:           "no mask",
			clusterIdentifier: MockSuccessDBClusterIdentifier,
			expectedSQL:       "",
			noMask:            true,
		},
		{
			casetag:           "intaractive",
			clusterIdentifier: MockSuccessDBClusterIdentifier,
			expectedSQL:       "-- nothing to do\nSELECT * FROM users LIMIT 5;",
			noMask:            true,
			cfg: &Config{
				TempCluster: TempDBClusterConfig{
					DBInstanceClass: "db.t3.small",
				},
				Interactive: true,
			},
			stdin: "SELECT * FROM users LIMIT 5;\nexit\n",
		},
	}
	for _, c := range cases {
		t.Run(c.casetag+c.clusterIdentifier, func(t *testing.T) {
			cleanup := setLogOutput(t)
			defer cleanup()
			svc := &mockRDSService{}
			e := &mockExecuter{}
			app := &App{
				rdsSvc:       svc,
				baseInterval: time.Millisecond,
				newExecuter: func(_ *Config, host string, _ int) (executer, error) {
					e.host = host
					return e, nil
				},
				stdin: io.NopCloser(strings.NewReader(c.stdin)),
			}
			if c.cfg == nil {
				app.cfg = DefaultConfig()
			} else {
				app.cfg = c.cfg
			}
			app.cfg.TempCluster.DBClusterIdentifier = c.clusterIdentifier
			if !c.noMask {
				app.cfg.SQLFile = "testdata/mask.sql"
			}
			require.NoError(t, app.cfg.Validate(), "config validate no error")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := app.Run(ctx, "mascaras-test")
			if c.errMsg == "" {
				require.NoError(t, err, "run no error")
			} else {
				require.EqualError(t, err, c.errMsg, "run expected error")
			}
			require.EqualValues(t, c.expectedSQL, e.executeSQL.String(), "sql check")
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
	flextime.Set(time.Date(2021, 06, 01, 0, 0, 0, 0, time.UTC))
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
			S3Prefix:       "/2021/06/01",
		},
	}
	require.EqualValues(t, expected, cfg)
}
