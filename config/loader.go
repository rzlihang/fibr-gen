package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadWorkbookConfig loads a workbook configuration from a YAML file.
func LoadWorkbookConfig(path string) (*WorkbookConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workbook config file: %w", err)
	}

	var cfg WorkbookConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse workbook config: %w", err)
	}

	return &cfg, nil
}

// LoadDataViewConfig loads a data view configuration from a YAML file.
func LoadDataViewConfig(path string) (*DataViewConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read data view config file: %w", err)
	}

	var cfg DataViewConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse data view config: %w", err)
	}

	return &cfg, nil
}

// LoadDataSourceConfig loads a data source configuration from a YAML file.
func LoadDataSourceConfig(path string) (*DataSourceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read data source config file: %w", err)
	}

	var cfg DataSourceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse data source config: %w", err)
	}

	return &cfg, nil
}

type BundleConfig struct {
	Workbook    *WorkbookConfig     `yaml:"workbook"`
	DataViews   []*DataViewConfig   `yaml:"dataViews"`
	DataSources []*DataSourceConfig `yaml:"dataSources"`
}

type DataSourcesBundle struct {
	DataSources []*DataSourceConfig `yaml:"dataSources"`
}

// LoadConfigBundle loads a single configuration bundle from a YAML file.
// It includes one workbook, and optional data views and data sources.
func LoadConfigBundle(path string) (*WorkbookConfig, map[string]*DataViewConfig, map[string]*DataSourceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read config bundle: %w", err)
	}

	var bundle BundleConfig
	if err := yaml.Unmarshal(data, &bundle); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse config bundle: %w", err)
	}

	if bundle.Workbook == nil {
		return nil, nil, nil, fmt.Errorf("config bundle missing workbook")
	}

	views := make(map[string]*DataViewConfig)
	for _, view := range bundle.DataViews {
		if view == nil || view.Name == "" {
			return nil, nil, nil, fmt.Errorf("data view config missing name")
		}
		if _, exists := views[view.Name]; exists {
			return nil, nil, nil, fmt.Errorf("duplicate data view name: %s", view.Name)
		}
		views[view.Name] = view
	}

	dataSources := make(map[string]*DataSourceConfig)
	for _, source := range bundle.DataSources {
		if source == nil || source.Name == "" {
			return nil, nil, nil, fmt.Errorf("data source config missing name")
		}
		if _, exists := dataSources[source.Name]; exists {
			return nil, nil, nil, fmt.Errorf("duplicate data source name: %s", source.Name)
		}
		dataSources[source.Name] = source
	}

	// Validate configuration
	registry := NewMemoryConfigRegistry(views, dataSources)
	validator := NewValidator(registry)

	for _, view := range views {
		if err := validator.ValidateDataView(view); err != nil {
			return nil, nil, nil, fmt.Errorf("data view '%s' validation failed: %w", view.Name, err)
		}
	}

	for _, source := range dataSources {
		if err := validator.ValidateDataSource(source); err != nil {
			return nil, nil, nil, fmt.Errorf("data source '%s' validation failed: %w", source.Name, err)
		}
	}

	if err := validator.ValidateWorkbook(bundle.Workbook); err != nil {
		return nil, nil, nil, fmt.Errorf("workbook validation failed: %w", err)
	}

	return bundle.Workbook, views, dataSources, nil
}

// LoadConfigBundleRaw parses a config bundle YAML without running any validation.
// Use this in the validate command to collect all issues rather than fail-fast.
func LoadConfigBundleRaw(path string) (*WorkbookConfig, map[string]*DataViewConfig, map[string]*DataSourceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read config bundle: %w", err)
	}

	var bundle BundleConfig
	if err := yaml.Unmarshal(data, &bundle); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse config bundle: %w", err)
	}

	if bundle.Workbook == nil {
		return nil, nil, nil, fmt.Errorf("config bundle missing workbook")
	}

	views := make(map[string]*DataViewConfig)
	for _, view := range bundle.DataViews {
		if view != nil && view.Name != "" {
			views[view.Name] = view
		}
	}

	dataSources := make(map[string]*DataSourceConfig)
	for _, source := range bundle.DataSources {
		if source != nil && source.Name != "" {
			dataSources[source.Name] = source
		}
	}

	return bundle.Workbook, views, dataSources, nil
}

// LoadDataSourcesBundle loads data source configurations from a YAML file.
// The file should contain a dataSources list.
func LoadDataSourcesBundle(path string) (map[string]*DataSourceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read data source bundle: %w", err)
	}

	var bundle DataSourcesBundle
	if err := yaml.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("failed to parse data source bundle: %w", err)
	}

	dataSources := make(map[string]*DataSourceConfig)
	for _, source := range bundle.DataSources {
		if source == nil || source.Name == "" {
			return nil, fmt.Errorf("data source config missing name")
		}
		if _, exists := dataSources[source.Name]; exists {
			return nil, fmt.Errorf("duplicate data source name: %s", source.Name)
		}
		dataSources[source.Name] = source
	}

	return dataSources, nil
}

// LoadAllConfigs loads all configurations from a directory.
// It expects subdirectories or naming conventions to distinguish types,
// or it tries to parse into different structs.
// For simplicity, let's assume a structure like:
// test/
//
//	workbooks/
//	  wb1.yaml
//	dataViews/
//	  vv1.yaml
//	datasources/
//	  ds1.yaml
func LoadAllConfigs(rootDir string) (map[string]*WorkbookConfig, map[string]*DataViewConfig, map[string]*DataSourceConfig, error) {
	workbooks := make(map[string]*WorkbookConfig)
	views := make(map[string]*DataViewConfig)
	dataSources := make(map[string]*DataSourceConfig)

	// Helper to walk and load
	walkDir := func(subDir string, loader func(string) error) error {
		path := filepath.Join(rootDir, subDir)
		entries, err := os.ReadDir(path)
		if os.IsNotExist(err) {
			return nil // Optional directory
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() && (filepath.Ext(entry.Name()) == ".yaml" || filepath.Ext(entry.Name()) == ".yml") {
				if err := loader(filepath.Join(path, entry.Name())); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// Load DataSources
	err := walkDir("datasources", func(f string) error {
		cfg, err := LoadDataSourceConfig(f)
		if err != nil {
			return err
		}
		dataSources[cfg.Name] = cfg
		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading datasources: %w", err)
	}

	// Load DataViews
	err = walkDir("dataViews", func(f string) error {
		cfg, err := LoadDataViewConfig(f)
		if err != nil {
			return err
		}
		views[cfg.Name] = cfg
		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading dataViews: %w", err)
	}

	// Load Workbooks
	err = walkDir("workbooks", func(f string) error {
		cfg, err := LoadWorkbookConfig(f)
		if err != nil {
			return err
		}
		workbooks[cfg.Id] = cfg
		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading workbooks: %w", err)
	}

	// Validate all configurations
	registry := NewMemoryConfigRegistry(views, dataSources)
	validator := NewValidator(registry)

	for _, view := range views {
		if err := validator.ValidateDataView(view); err != nil {
			return nil, nil, nil, fmt.Errorf("data view '%s' validation failed: %w", view.Name, err)
		}
	}

	for _, source := range dataSources {
		if err := validator.ValidateDataSource(source); err != nil {
			return nil, nil, nil, fmt.Errorf("data source '%s' validation failed: %w", source.Name, err)
		}
	}

	for _, wb := range workbooks {
		if err := validator.ValidateWorkbook(wb); err != nil {
			return nil, nil, nil, fmt.Errorf("workbook '%s' validation failed: %w", wb.Name, err)
		}
	}

	return workbooks, views, dataSources, nil
}
