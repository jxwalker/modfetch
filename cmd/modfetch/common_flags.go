package main

import "flag"

type commonConfigLogFlags struct {
	configPath *string
	logLevel   *string
	jsonOut    *bool
}

func addCommonConfigLogFlags(fs *flag.FlagSet, jsonUsage string) commonConfigLogFlags {
	if jsonUsage == "" {
		jsonUsage = "json logs"
	}
	return commonConfigLogFlags{
		configPath: fs.String("config", "", "Path to YAML config file"),
		logLevel:   fs.String("log-level", "info", "log level"),
		jsonOut:    fs.Bool("json", false, jsonUsage),
	}
}

func addConfigPathFlag(fs *flag.FlagSet) *string {
	return fs.String("config", "", "Path to YAML config file")
}
