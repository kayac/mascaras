package mascaras

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
)

const (
	MockSuccessDBClusterIdentifier               = "mascaras-test"
	MockFailureRestoreDBClusterIdentifier        = "mascaras-failure-restore-test"
	MockFailureCreateInstanceDBClusterIdentifier = "mascaras-failure-create-instance-test"
	MockFailureExecuteSQLDBClusterIdentifier     = "mascaras-failure-exec-sql-test"
	MockFailureCreateSnapshotDBClusterIdentifier = "mascaras-failure-create-snapshot-test"
	MockFailureExportTaskIdentifier              = "mascaras-failure-export-task-test"
)

type mockRDSService struct {
	rdsiface.RDSAPI
	dbClusterCreateTime  time.Time
	dbInstanceCreateTime time.Time
	snapshotCreateTime   time.Time
	isCreateCluster      bool
	isCreateInstance     bool
	isDeleteCluster      bool
	isDeleteInstance     bool
}

const (
	dbInstanceARNPrefix        = "arn:aws:rds:ap-northeast-1:000000000000:db:"
	dbClusterARNPrefix         = "arn:aws:rds:ap-northeast-1:000000000000:cluster:"
	dbClusterEndpointSuffix    = ".cluster-000000000000.ap-northeast-1.rds.amazonaws.com"
	dbClusterSnapshotARNPrefix = "arn:aws:rds:ap-northeast-1:000000000000:cluster-snapshot:"
)

func (svc *mockRDSService) RestoreDBClusterToPointInTimeWithContext(
	ctx context.Context,
	input *rds.RestoreDBClusterToPointInTimeInput,
	_ ...request.Option,
) (*rds.RestoreDBClusterToPointInTimeOutput, error) {
	if *input.DBClusterIdentifier == MockFailureRestoreDBClusterIdentifier {
		return nil, errors.New("failure RestoreDBClusterToPointInTimeWithContext")
	}
	svc.dbClusterCreateTime = time.Now()
	svc.isCreateCluster = true
	output := &rds.RestoreDBClusterToPointInTimeOutput{
		DBCluster: &rds.DBCluster{
			DBClusterArn: aws.String(dbClusterARNPrefix + *input.DBClusterIdentifier),
			Port:         aws.Int64(3306),
			Engine:       aws.String("aurora-test"),
		},
	}
	return output, nil
}

func (svc *mockRDSService) CreateDBInstanceWithContext(
	ctx context.Context,
	input *rds.CreateDBInstanceInput,
	_ ...request.Option,
) (*rds.CreateDBInstanceOutput, error) {
	if *input.DBClusterIdentifier == MockFailureCreateInstanceDBClusterIdentifier {
		return nil, errors.New("failure CreateDBInstanceWithContext")
	}
	svc.dbInstanceCreateTime = time.Now()
	svc.isCreateInstance = true
	output := &rds.CreateDBInstanceOutput{
		DBInstance: &rds.DBInstance{
			DBClusterIdentifier: input.DBClusterIdentifier,
			DBInstanceArn:       aws.String(dbInstanceARNPrefix + *input.DBInstanceIdentifier),
		},
	}
	return output, nil
}

func (svc *mockRDSService) CreateDBClusterSnapshotWithContext(
	ctx context.Context,
	input *rds.CreateDBClusterSnapshotInput,
	_ ...request.Option,
) (*rds.CreateDBClusterSnapshotOutput, error) {
	if *input.DBClusterIdentifier == MockFailureCreateSnapshotDBClusterIdentifier {
		return nil, errors.New("failure CreateDBClusterSnapshotWithContext")
	}
	svc.snapshotCreateTime = time.Now()
	output := &rds.CreateDBClusterSnapshotOutput{
		DBClusterSnapshot: &rds.DBClusterSnapshot{
			DBClusterSnapshotArn: aws.String(dbClusterSnapshotARNPrefix + *input.DBClusterSnapshotIdentifier),
		},
	}
	return output, nil
}

func (svc *mockRDSService) StartExportTaskWithContext(
	ctx context.Context,
	input *rds.StartExportTaskInput,
	_ ...request.Option,
) (*rds.StartExportTaskOutput, error) {
	output := &rds.StartExportTaskOutput{}
	if *input.ExportTaskIdentifier == MockFailureExportTaskIdentifier {
		output.FailureCause = aws.String("task identifer is invalid")
		return output, errors.New("failure StartExportTaskWithContext")
	}
	return output, nil
}

func (svc *mockRDSService) DescribeDBInstancesWithContext(
	ctx context.Context,
	input *rds.DescribeDBInstancesInput,
	_ ...request.Option,
) (*rds.DescribeDBInstancesOutput, error) {
	status := "creating"
	if time.Since(svc.dbInstanceCreateTime) > 8*time.Millisecond {
		status = "available"
	}
	output := &rds.DescribeDBInstancesOutput{
		DBInstances: []*rds.DBInstance{
			{
				DBInstanceIdentifier: input.DBInstanceIdentifier,
				DBInstanceStatus:     aws.String(status),
			},
		},
	}
	return output, nil
}

func (svc *mockRDSService) DescribeDBClustersWithContext(
	ctx context.Context,
	input *rds.DescribeDBClustersInput,
	_ ...request.Option,
) (*rds.DescribeDBClustersOutput, error) {
	status := "creating"
	if time.Since(svc.dbClusterCreateTime) > 5*time.Millisecond {
		status = "available"
	}
	latestRestorableTime := time.Now().Add(-5 * time.Millisecond).UTC()
	output := &rds.DescribeDBClustersOutput{
		DBClusters: []*rds.DBCluster{
			{
				DBClusterIdentifier:  input.DBClusterIdentifier,
				Status:               aws.String(status),
				Port:                 aws.Int64(3306),
				LatestRestorableTime: aws.Time(latestRestorableTime),
			},
		},
	}
	return output, nil
}

func (svc *mockRDSService) DescribeDBClusterEndpointsWithContext(
	ctx context.Context,
	input *rds.DescribeDBClusterEndpointsInput,
	_ ...request.Option,
) (*rds.DescribeDBClusterEndpointsOutput, error) {
	status := "creating"
	if time.Since(svc.dbClusterCreateTime) > 8*time.Millisecond {
		status = "available"
	}
	output := &rds.DescribeDBClusterEndpointsOutput{
		DBClusterEndpoints: []*rds.DBClusterEndpoint{
			{
				Endpoint:     aws.String(*input.DBClusterIdentifier + dbClusterEndpointSuffix),
				EndpointType: aws.String("WRITER"),
				Status:       aws.String(status),
			},
		},
	}
	return output, nil
}

func (svc *mockRDSService) DescribeDBClusterSnapshotsWithContext(
	ctx context.Context,
	input *rds.DescribeDBClusterSnapshotsInput,
	_ ...request.Option,
) (*rds.DescribeDBClusterSnapshotsOutput, error) {
	status := "creating"
	since := time.Since(svc.snapshotCreateTime)
	if since > 10*time.Millisecond {
		status = "available"
	}
	percent := since * 100 / (10 * time.Millisecond)
	if percent > 100 {
		percent = 100
	}
	output := &rds.DescribeDBClusterSnapshotsOutput{
		DBClusterSnapshots: []*rds.DBClusterSnapshot{
			{
				PercentProgress: aws.Int64(int64(percent)),
				Status:          aws.String(status),
			},
		},
	}
	return output, nil
}

func (svc *mockRDSService) DeleteDBCluster(
	input *rds.DeleteDBClusterInput,
) (*rds.DeleteDBClusterOutput, error) {
	svc.isDeleteCluster = true
	return &rds.DeleteDBClusterOutput{
		DBCluster: &rds.DBCluster{
			DBClusterArn: aws.String(dbClusterARNPrefix + *input.DBClusterIdentifier),
		},
	}, nil
}

func (svc *mockRDSService) DeleteDBInstance(
	input *rds.DeleteDBInstanceInput,
) (*rds.DeleteDBInstanceOutput, error) {
	svc.isDeleteInstance = true
	return &rds.DeleteDBInstanceOutput{
		DBInstance: &rds.DBInstance{
			DBInstanceArn: aws.String(dbInstanceARNPrefix + *input.DBInstanceIdentifier),
		},
	}, nil
}

type mockExecuter struct {
	host            string
	executeSQL      strings.Builder
	lastExecuteTime time.Time
}

func (e *mockExecuter) ExecuteContext(_ context.Context, reader io.Reader) error {
	if strings.Contains(e.host, MockFailureExecuteSQLDBClusterIdentifier) {
		return errors.New("failed ExecuteContext")
	}
	bs, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	e.executeSQL.WriteString(string(bs))
	e.lastExecuteTime = time.Now().UTC()
	return nil
}
func (e *mockExecuter) LastExecuteTime() time.Time {
	return e.lastExecuteTime
}

func (e *mockExecuter) SetTableSelectHook(func(string, string))   {}
func (e *mockExecuter) SetExecuteHook(func(string, int64, int64)) {}

func (e *mockExecuter) Close() error {
	return nil
}
