package main

import (
	"context"
	"database/sql"
	"fibr-gen/config"
	"fibr-gen/core"
	"flag"
	"fmt"
	"io"
	"log/slog"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

func runGenerate(output io.Writer, args []string) error {
	flags := flag.NewFlagSet("fibr-gen generate", flag.ContinueOnError)
	flags.SetOutput(output)

	flags.Usage = func() {
		fmt.Fprintf(output, "Usage: fibr-gen generate [flags]\n\nFlags:\n")
		flags.PrintDefaults()
	}

	var (
		configFile     string
		dataSourceFile string
		templateDir    string
		outputDir      string
		fetcherType    string
		dbDSN          string
		csvDir         string
		s3Bucket       string
		s3Prefix       string
	)

	flags.StringVar(&configFile, "config", "./test/config.yaml", "Path to configuration bundle")
	flags.StringVar(&configFile, "c", "./test/config.yaml", "Path to configuration bundle (short)")
	flags.StringVar(&dataSourceFile, "datasources", "", "Path to data source bundle (optional)")
	flags.StringVar(&templateDir, "templates", "./test/templates", "Template group directory")
	flags.StringVar(&templateDir, "t", "./test/templates", "Template group directory (short)")
	flags.StringVar(&outputDir, "output", "./test/output", "Directory for output files")
	flags.StringVar(&outputDir, "o", "./test/output", "Directory for output files (short)")
	flags.StringVar(&fetcherType, "fetcher", "csv", "Data fetcher type: csv, dynamodb, mysql, postgres")
	flags.StringVar(&fetcherType, "f", "csv", "Data fetcher type (short)")
	flags.StringVar(&dbDSN, "db-dsn", "", "Database connection string (DSN) for mysql/postgres")
	flags.StringVar(&csvDir, "csv-dir", "./test/data_csv", "Directory containing CSV files for csv fetcher")
	flags.StringVar(&s3Bucket, "s3-bucket", "", "S3 bucket name for uploading output")
	flags.StringVar(&s3Prefix, "s3-prefix", "fibr-gen-output", "S3 prefix (folder) for uploaded files")

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	slog.Info("Loading configuration bundle", "file", configFile)
	wbConf, views, dataSources, err := config.LoadConfigBundle(configFile)
	if err != nil {
		return err
	}
	if dataSourceFile != "" {
		slog.Info("Loading data source bundle", "file", dataSourceFile)
		dataSources, err = config.LoadDataSourcesBundle(dataSourceFile)
		if err != nil {
			return err
		}
	}
	if len(dataSources) > 0 {
		slog.Info("Loaded data sources", "count", len(dataSources))
	}

	var fetcher core.DataFetcher

	switch fetcherType {
	case "dynamodb":
		slog.Info("Initializing DynamoDB Data Fetcher")
		cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
		if err != nil {
			return fmt.Errorf("unable to load AWS SDK config: %w", err)
		}
		fetcher = core.NewDynamoDBDataFetcher(cfg)
	case "mysql", "postgres":
		if dbDSN == "" {
			return fmt.Errorf("db-dsn is required for %s fetcher", fetcherType)
		}
		slog.Info("Initializing SQL Data Fetcher", "type", fetcherType)
		db, err := sql.Open(fetcherType, dbDSN)
		if err != nil {
			return fmt.Errorf("failed to open db connection: %w", err)
		}
		if err := db.Ping(); err != nil {
			return fmt.Errorf("failed to ping db: %w", err)
		}
		fetcher = core.NewSQLDataFetcher(db, fetcherType)
	default:
		slog.Info("Initializing CSV Data Fetcher", "dir", csvDir)
		fetcher = core.NewCsvDataFetcher(csvDir)
	}

	slog.Info("Processing Workbook", "name", wbConf.Name, "id", wbConf.Id)
	configRegistry := config.NewMemoryConfigRegistry(views, dataSources)
	ctx := core.NewGenerationContext(wbConf, configRegistry, fetcher, map[string]string{"env": "dev"})
	generator := core.NewGenerator(ctx)
	if err := generator.Generate(templateDir, outputDir); err != nil {
		return fmt.Errorf("generate workbook %s: %w", wbConf.Name, err)
	}
	slog.Info("Successfully generated", "name", wbConf.Name)

	if s3Bucket != "" {
		slog.Info("Starting S3 upload", "bucket", s3Bucket, "prefix", s3Prefix)
		cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
		if err != nil {
			return fmt.Errorf("unable to load AWS SDK config for S3: %w", err)
		}
		uploader := core.NewS3Uploader(cfg, s3Bucket, s3Prefix)
		if err := uploader.UploadDirectory(outputDir); err != nil {
			return fmt.Errorf("failed to upload output to s3: %w", err)
		}
		slog.Info("Successfully uploaded to S3")
	}

	return nil
}
