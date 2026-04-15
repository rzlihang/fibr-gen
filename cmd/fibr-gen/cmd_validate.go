package main

import (
	"fibr-gen/config"
	"fibr-gen/core"
	"flag"
	"fmt"
	"io"
	"log/slog"
)

func runValidate(output io.Writer, args []string) error {
	flags := flag.NewFlagSet("fibr-gen validate", flag.ContinueOnError)
	flags.SetOutput(output)

	flags.Usage = func() {
		fmt.Fprintf(output, "Usage: fibr-gen validate [flags]\n\nFlags:\n")
		flags.PrintDefaults()
	}

	var (
		configFile     string
		dataSourceFile string
		templateDir    string
	)

	flags.StringVar(&configFile, "config", "./test/config.yaml", "Path to configuration bundle")
	flags.StringVar(&configFile, "c", "./test/config.yaml", "Path to configuration bundle (short)")
	flags.StringVar(&dataSourceFile, "datasources", "", "Path to data source bundle (optional)")
	flags.StringVar(&templateDir, "templates", "./test/templates", "Template group directory")
	flags.StringVar(&templateDir, "t", "./test/templates", "Template group directory (short)")

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("Loading config", "file", configFile)
	wbConf, views, dataSources, err := config.LoadConfigBundleRaw(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if dataSourceFile != "" {
		extra, err := config.LoadDataSourcesBundle(dataSourceFile)
		if err != nil {
			return fmt.Errorf("failed to load data sources: %w", err)
		}
		for k, v := range extra {
			dataSources[k] = v
		}
	}

	slog.Info("Loading template", "dir", templateDir, "file", wbConf.Template)

	tv := core.NewTemplateValidator(wbConf, views, templateDir)
	issues := tv.Validate()

	// Check DataSource completeness (offline — no connection attempt)
	for _, ds := range dataSources {
		if ds.DSN == "" {
			issues = append(issues, core.ValidationIssue{
				Level:    core.IssueLevelWarn,
				Category: "config",
				Message:  fmt.Sprintf("DataSource %q has empty DSN", ds.Name),
			})
		}
	}

	errorCount := 0
	warnCount := 0
	for _, iss := range issues {
		fmt.Fprintln(output, iss.String())
		switch iss.Level {
		case core.IssueLevelError:
			errorCount++
		case core.IssueLevelWarn:
			warnCount++
		}
	}

	fmt.Fprintf(output, "INFO   Validation complete: %d error(s), %d warning(s)\n", errorCount, warnCount)

	if errorCount > 0 {
		return fmt.Errorf("validation failed with %d error(s)", errorCount)
	}
	return nil
}
