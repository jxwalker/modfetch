package main

import (
	"flag"
	"testing"
)

func TestAddCommonConfigLogFlags(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "")
	if err := fs.Parse([]string{"--config", "/tmp/config.yml", "--log-level", "debug", "--json"}); err != nil {
		t.Fatalf("parse common flags: %v", err)
	}
	if *common.configPath != "/tmp/config.yml" {
		t.Fatalf("config path = %q", *common.configPath)
	}
	if *common.logLevel != "debug" {
		t.Fatalf("log level = %q", *common.logLevel)
	}
	if !*common.jsonOut {
		t.Fatal("expected json flag to be true")
	}
}

func TestAddConfigPathFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfgPath := addConfigPathFlag(fs)
	if err := fs.Parse([]string{"--config", "/tmp/config.yml"}); err != nil {
		t.Fatalf("parse config flag: %v", err)
	}
	if *cfgPath != "/tmp/config.yml" {
		t.Fatalf("config path = %q", *cfgPath)
	}
}
