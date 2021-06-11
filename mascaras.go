package mascaras

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/lestrrat-go/backoff/v2"
	"github.com/mashiike/mysqlbatch"
)

type App struct {
	rdsSvc       rdsiface.RDSAPI
	cfg          *Config
	baseInterval time.Duration
	newExecuter  func(cfg *Config, dbCluster *rds.DBCluster, dbClusterEndpoint *rds.DBClusterEndpoint) (executer, error)
}

func New(cfg *Config, cfgs ...*aws.Config) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	session, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, err
	}
	return &App{
		rdsSvc:       rds.New(session, cfgs...),
		cfg:          cfg,
		newExecuter:  defaultNewExecuter,
		baseInterval: time.Minute,
	}, err
}

func readSQL(location string) (string, error) {
	r, err := openLocation(location)
	if err != nil {
		return "", err
	}
	defer r.Close()
	bs, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

type cleanupInfo struct {
	tempDBClusterIdentifier  *string
	tempDBInstanceIdentifier *string
}

type executer interface {
	ExecuteContext(context.Context, io.Reader) error
	LastExecuteTime() time.Time
	SetTableSelectHook(func(query, table string))
	SetExecuteHook(func(query string, rowsAffected int64, lastInsertId int64))
	Close() error
}

func defaultNewExecuter(cfg *Config, dbCluster *rds.DBCluster, dbClusterEndpoint *rds.DBClusterEndpoint) (executer, error) {
	mysqlConfig := &mysqlbatch.Config{
		User:     cfg.DBUserName,
		Host:     *dbClusterEndpoint.Endpoint,
		Password: cfg.DBUserPassword,
		Port:     int(*dbCluster.Port),
		Database: cfg.Database,
	}
	executer, err := mysqlbatch.New(mysqlConfig)
	if err != nil {
		return nil, err
	}
	return executer, nil
}

func (app *App) Run(ctx context.Context, maskSQLFile, sourceDBClusterIdentifier string) error {
	if maskSQLFile == "" {
		maskSQLFile = app.cfg.SQLFile
	}
	if sourceDBClusterIdentifier == "" {
		sourceDBClusterIdentifier = app.cfg.SourceDBClusterIdentifier
	}
	if maskSQLFile == "" && sourceDBClusterIdentifier == "" {
		return errors.New("sql file and source db cluster is required")
	}

	maskSQL, err := readSQL(maskSQLFile)
	if err != nil {
		return err
	}
	log.Println("[debug] sql:", maskSQL)
	tempDBClusterIdentifier := app.cfg.TempCluster.DBClusterIdentifier
	if tempDBClusterIdentifier == "" {
		rstr, err := randstr(10)
		if err != nil {
			return err
		}
		tempDBClusterIdentifier = app.cfg.TempCluster.DBClusterIdentifierPrefix + rstr
	}
	restoreOutput, err := app.rdsSvc.RestoreDBClusterToPointInTimeWithContext(ctx, &rds.RestoreDBClusterToPointInTimeInput{
		SourceDBClusterIdentifier: &sourceDBClusterIdentifier,
		DBClusterIdentifier:       &tempDBClusterIdentifier,
		RestoreType:               aws.String("copy-on-write"),
		UseLatestRestorableTime:   aws.Bool(true),
		VpcSecurityGroupIds:       aws.StringSlice(app.cfg.TempCluster.securityGroupIDs()),
	})
	if err != nil {
		return fmt.Errorf("RestoreDBClusterToPointInTime:%w", err)
	}
	cleanupInfo := &cleanupInfo{
		tempDBClusterIdentifier: &tempDBClusterIdentifier,
	}
	defer func() {
		if err := app.cleanup(cleanupInfo); err != nil {
			log.Printf("[error] cleanup failed: %s", err.Error())
		}
	}()
	log.Printf("[info] cloned db cluster: %s\n", *restoreOutput.DBCluster.DBClusterArn)
	tempDBCluster, err := app.waitDBClusterAvailable(ctx, tempDBClusterIdentifier)
	if err != nil {
		return err
	}
	tempDBInstanceIdentifier := tempDBClusterIdentifier + "-instance"

	createInstanceOutput, err := app.rdsSvc.CreateDBInstanceWithContext(ctx, &rds.CreateDBInstanceInput{
		DBClusterIdentifier:  &tempDBClusterIdentifier,
		DBInstanceIdentifier: &tempDBInstanceIdentifier,
		DBInstanceClass:      &app.cfg.TempCluster.DBInstanceClass,
		Engine:               tempDBCluster.Engine,
		PubliclyAccessible:   &app.cfg.TempCluster.PubliclyAccessible,
	})
	if err != nil {
		return err
	}
	cleanupInfo.tempDBInstanceIdentifier = &tempDBInstanceIdentifier
	log.Printf("[info] create db instance: %s\n", *createInstanceOutput.DBInstance.DBInstanceArn)
	_, err = app.waitDBInstanceAvailable(ctx, tempDBInstanceIdentifier)
	if err != nil {
		return err
	}
	tempDBClusterEndpoint, err := app.waitDBClusterEndpointAvailable(ctx, tempDBClusterIdentifier)
	if err != nil {
		return err
	}

	maskedTime, err := app.executeSQL(ctx, maskSQL, maskSQLFile, tempDBCluster, tempDBClusterEndpoint)
	if err != nil {
		return err
	}
	if err := app.waitDBClusterLatestRestorableTime(ctx, tempDBClusterIdentifier, maskedTime); err != nil {
		return err
	}
	snapshotIdentifer := tempDBClusterIdentifier + "-snapshot"
	_, err = app.rdsSvc.CreateDBClusterSnapshotWithContext(ctx, &rds.CreateDBClusterSnapshotInput{
		DBClusterIdentifier:         &tempDBClusterIdentifier,
		DBClusterSnapshotIdentifier: &snapshotIdentifer,
	})
	if err != nil {
		return err
	}
	if !app.cfg.EnableExportTask {
		return nil
	}
	if err := app.cleanup(cleanupInfo); err != nil {
		return err
	}
	log.Println("[info] snapshot export to s3 enable")
	snapshot, err := app.waitDBClusterSnapshot(ctx, snapshotIdentifer)
	if err != nil {
		return err
	}
	taskIdentifier := app.cfg.ExportTask.TaskIdentifier
	if taskIdentifier == "" {
		taskIdentifier = snapshotIdentifer + "-export-task"
	}
	log.Printf("[info] start export task, export task identifier=%s\n", taskIdentifier)
	taskOutput, err := app.rdsSvc.StartExportTaskWithContext(ctx, &rds.StartExportTaskInput{
		ExportTaskIdentifier: &taskIdentifier,
		IamRoleArn:           &app.cfg.ExportTask.IAMRoleArn,
		KmsKeyId:             &app.cfg.ExportTask.KMSKeyId,
		S3BucketName:         &app.cfg.ExportTask.S3Bucket,
		S3Prefix:             aws.String(app.cfg.ExportTask.S3Prefix),
		ExportOnly:           aws.StringSlice(app.cfg.ExportTask.exportOnly()),
		SourceArn:            snapshot.DBClusterSnapshotArn,
	})
	if taskOutput.FailureCause != nil {
		log.Printf("[warn] failure cause: %s\n", *taskOutput.FailureCause)
	}
	if taskOutput.WarningMessage != nil {
		log.Printf("[warn] %s\n", *taskOutput.WarningMessage)
	}
	if err != nil {
		return err
	}
	log.Println("[info] all finish.")
	return nil
}
func (app *App) executeSQL(ctx context.Context, maskSQL, maskSQLLoc string, dbCluster *rds.DBCluster, dbClusterEndpoint *rds.DBClusterEndpoint) (time.Time, error) {
	executer, err := app.newExecuter(app.cfg, dbCluster, dbClusterEndpoint)
	if err != nil {
		return time.Time{}, err
	}
	defer executer.Close()
	executer.SetTableSelectHook(func(query, table string) {
		log.Printf("[info] Query: %s\n%s\n", query, table)
	})
	log.Printf("[info] start do sql `%s`\n", maskSQLLoc)
	if err := executer.ExecuteContext(ctx, strings.NewReader(maskSQL)); err != nil {
		return executer.LastExecuteTime(), err
	}
	log.Println("[info] end do sql")
	return executer.LastExecuteTime(), nil
}

func (app *App) wait(ctx context.Context, estimateTime time.Duration, action func() bool) error {
	constantPolicy := backoff.NewConstantPolicy(
		backoff.WithInterval(app.baseInterval),
		backoff.WithJitterFactor(0.05),
		backoff.WithMaxRetries(int(estimateTime/app.baseInterval)),
	)
	c := constantPolicy.Start(ctx)
	for backoff.Continue(c) {
		if action() {
			return nil
		}
	}
	exPolicy := backoff.Exponential(
		backoff.WithMinInterval(app.baseInterval),
		backoff.WithMaxInterval(5*app.baseInterval),
		backoff.WithJitterFactor(0.05),
	)
	c = exPolicy.Start(ctx)
	if !backoff.Continue(c) {
		return nil
	}
	for backoff.Continue(c) {
		if action() {
			return nil
		}
	}
	return errors.New("failed to wait available, timeout")
}

func (app *App) waitDBClusterAvailable(ctx context.Context, dbClusterIdentifeier string) (dbCluster *rds.DBCluster, err error) {
	log.Printf("[info] wait db cluster `%s` status available...\n", dbClusterIdentifeier)

	act := func() bool {
		var output *rds.DescribeDBClustersOutput
		output, err = app.rdsSvc.DescribeDBClustersWithContext(ctx, &rds.DescribeDBClustersInput{
			DBClusterIdentifier: &dbClusterIdentifeier,
		})
		if err != nil {
			return true
		}
		if len(output.DBClusters) == 0 {
			err = fmt.Errorf("db cluster `%s` not found", dbClusterIdentifeier)
			return true
		}
		if strings.ToLower(*output.DBClusters[0].Status) == "available" {
			log.Printf("[info] db cluster status is %s!\n", *output.DBClusters[0].Status)
			err = nil
			dbCluster = output.DBClusters[0]
			return true
		}
		log.Printf("[info] now db cluster status is %s ...\n", *output.DBClusters[0].Status)
		return false
	}
	err = app.wait(ctx, 5*time.Minute, act)
	return
}

func (app *App) waitDBInstanceAvailable(ctx context.Context, dbInstanceIdentifeier string) (dbInstance *rds.DBInstance, err error) {
	log.Printf("[info] wait db instance `%s` status available...\n", dbInstanceIdentifeier)
	act := func() bool {
		var output *rds.DescribeDBInstancesOutput
		output, err = app.rdsSvc.DescribeDBInstancesWithContext(ctx, &rds.DescribeDBInstancesInput{
			DBInstanceIdentifier: &dbInstanceIdentifeier,
		})
		if err != nil {
			return true
		}
		if len(output.DBInstances) == 0 {
			err = fmt.Errorf("db instance `%s` not found", dbInstanceIdentifeier)
			return true
		}
		if strings.ToLower(*output.DBInstances[0].DBInstanceStatus) == "available" {
			log.Printf("[info] db instance status is %s!\n", *output.DBInstances[0].DBInstanceStatus)
			dbInstance = output.DBInstances[0]
			err = nil
			return true
		}
		log.Printf("[info] now db instance status is %s ...\n", *output.DBInstances[0].DBInstanceStatus)
		return false
	}
	err = app.wait(ctx, 5*time.Minute, act)
	return
}

func (app *App) waitDBClusterEndpointAvailable(ctx context.Context, dbClusterIdentifeier string) (dbClusterEndpoint *rds.DBClusterEndpoint, err error) {
	log.Printf("[info] wait db endpoints `%s` status available...\n", dbClusterIdentifeier)
	act := func() bool {
		var output *rds.DescribeDBClusterEndpointsOutput
		output, err = app.rdsSvc.DescribeDBClusterEndpointsWithContext(ctx, &rds.DescribeDBClusterEndpointsInput{
			DBClusterIdentifier: &dbClusterIdentifeier,
			Filters: []*rds.Filter{
				{
					Name:   aws.String("db-cluster-endpoint-type"),
					Values: []*string{aws.String("WRITER")},
				},
			},
		})
		if err != nil {
			return true
		}
		if len(output.DBClusterEndpoints) == 0 {
			err = fmt.Errorf("db cluster endpoints `%s` not found", dbClusterIdentifeier)
			return true
		}
		if strings.ToLower(*output.DBClusterEndpoints[0].Status) == "available" {
			log.Printf("[info] db cluster endpoint status is %s!\n", *output.DBClusterEndpoints[0].Status)
			dbClusterEndpoint = output.DBClusterEndpoints[0]
			err = nil
			return true
		}
		log.Printf("[info] now db cluster endpoint status is %s ...\n", *output.DBClusterEndpoints[0].Status)
		return false
	}
	err = app.wait(ctx, 5*time.Minute, act)
	return
}

func (app *App) waitDBClusterLatestRestorableTime(ctx context.Context, dbClusterIdentifeier string, maskedTime time.Time) (err error) {
	log.Printf("[info] wait db cluster `%s` LatestRestorableTime past masked time `%s`...\n", dbClusterIdentifeier, maskedTime.Format(time.RFC3339))
	act := func() bool {
		var output *rds.DescribeDBClustersOutput
		output, err = app.rdsSvc.DescribeDBClustersWithContext(ctx, &rds.DescribeDBClustersInput{
			DBClusterIdentifier: &dbClusterIdentifeier,
		})
		if err != nil {
			return true
		}
		if len(output.DBClusters) == 0 {
			err = fmt.Errorf("db cluster `%s` not found", dbClusterIdentifeier)
			return true
		}
		latestRestorableTime := output.DBClusters[0].LatestRestorableTime
		if latestRestorableTime == nil {
			return false
		}
		if latestRestorableTime.After(maskedTime) {
			log.Printf("[info] db cluster LatestRestorableTime=%s, complete!\n", latestRestorableTime.Format(time.RFC3339))
			return true
		}
		log.Printf("[info] now db cluster LatestRestorableTime=%s\n", latestRestorableTime.Format(time.RFC3339))
		return false
	}
	err = app.wait(ctx, 5*time.Minute, act)
	return
}

func (app *App) waitDBClusterSnapshot(ctx context.Context, dbClusterSnapshotIdentifeier string) (dbClusterSnapshot *rds.DBClusterSnapshot, err error) {
	log.Printf("[info] wait db cluster snapshot `%s` status available...\n", dbClusterSnapshotIdentifeier)

	act := func() bool {
		var output *rds.DescribeDBClusterSnapshotsOutput
		output, err = app.rdsSvc.DescribeDBClusterSnapshotsWithContext(ctx, &rds.DescribeDBClusterSnapshotsInput{
			DBClusterSnapshotIdentifier: &dbClusterSnapshotIdentifeier,
		})
		if err != nil {
			return true
		}
		if len(output.DBClusterSnapshots) == 0 {
			err = fmt.Errorf("db cluster snapshot `%s` not found", dbClusterSnapshotIdentifeier)
			return true
		}
		if strings.ToLower(*output.DBClusterSnapshots[0].Status) == "available" {
			log.Printf(
				"[info] db cluster snapshot status is %s! progress=%d%%\n",
				*output.DBClusterSnapshots[0].Status,
				*output.DBClusterSnapshots[0].PercentProgress,
			)
			err = nil
			dbClusterSnapshot = output.DBClusterSnapshots[0]
			return true
		}
		log.Printf(
			"[info] db cluster status snapshot is %s... progress=%d%%\n",
			*output.DBClusterSnapshots[0].Status,
			*output.DBClusterSnapshots[0].PercentProgress,
		)
		return false
	}
	err = app.wait(ctx, 5*time.Minute, act)
	return
}

func (app *App) cleanup(info *cleanupInfo) error {
	log.Println("[info] start cleanup ...")
	if info.tempDBInstanceIdentifier != nil {
		output, err := app.rdsSvc.DeleteDBInstance(&rds.DeleteDBInstanceInput{
			DBInstanceIdentifier: info.tempDBInstanceIdentifier,
			SkipFinalSnapshot:    aws.Bool(true),
		})
		if err != nil {
			return err
		}
		log.Printf("[info] delete temp db instance:%s\n", *output.DBInstance.DBInstanceArn)
		info.tempDBInstanceIdentifier = nil
	}

	if info.tempDBClusterIdentifier != nil {
		output, err := app.rdsSvc.DeleteDBCluster(&rds.DeleteDBClusterInput{
			DBClusterIdentifier: info.tempDBClusterIdentifier,
			SkipFinalSnapshot:   aws.Bool(true),
		})
		if err != nil {
			return err
		}
		log.Printf("[info] delete temp db cluster:%s\n", *output.DBCluster.DBClusterArn)
		info.tempDBClusterIdentifier = nil
	}
	log.Println("[info] finish cleanup")
	return nil
}
