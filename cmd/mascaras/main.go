package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hashicorp/logutils"
	"github.com/kayac/mascaras"
	"github.com/mashiike/didumean"
)

var filter = &logutils.LevelFilter{
	Levels:   []logutils.LogLevel{"debug", "info", "warn", "error"},
	MinLevel: logutils.LogLevel("info"),
	Writer:   os.Stderr,
}

var (
	Version  = "0.0.0"
	Revision = ""
)

const envPrefix = "MASCARAS_"

func main() {
	var debug, showHelp, showVersion bool
	var configFile string
	cfg := mascaras.DefaultConfig()
	cfg.SetFlags(flag.CommandLine)
	flag.BoolVar(&debug, "debug", false, "enable debug log")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&showHelp, "help", false, "show help")
	flag.StringVar(&configFile, "config", "", "config file path")
	flag.VisitAll(func(f *flag.Flag) {
		name := envPrefix + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		if v, exists := os.LookupEnv(name); exists {
			f.Value.Set(v)
		}
	})
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: mascaras [options] <mask sql file> <source db cluster identifier>")
		fmt.Fprintf(flag.CommandLine.Output(), "\t can use %s env prefix\n", envPrefix)
		flag.PrintDefaults()
	}
	didumean.Parse()

	if showVersion {
		fmt.Printf("mascaras version=v%s revision=%s\n", Version, Revision)
		return
	}
	if showHelp {
		flag.Usage()
		return
	}
	sqlFile := os.Getenv(envPrefix + "SQL_FILE")
	sourceDBClusterIdentifier := os.Getenv(envPrefix + "SRC_DB_CLUSTER")
	if flag.NArg() == 2 {
		sqlFile = flag.Arg(0)
		sourceDBClusterIdentifier = flag.Arg(1)
	}
	if sqlFile == "" || sourceDBClusterIdentifier == "" {
		flag.Usage()
		log.Fatalln("<source db cluster identifier> and <sql file>  is required")
	}

	if debug {
		filter.MinLevel = logutils.LogLevel("debug")
	}
	if configFile != "" {
		o, err := mascaras.LoadConfig(configFile)
		if err != nil {
			log.Fatalf("[error] load config %s", err.Error())
		}
		cfg = o.MergeIn(cfg)
	}
	log.SetOutput(filter)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM|syscall.SIGHUP|syscall.SIGINT)
	defer stop()

	app, err := mascaras.New(cfg)
	if err != nil {
		log.Fatalf("[error] %v\n", err)
	}
	err = app.Run(ctx, sqlFile, sourceDBClusterIdentifier)
	switch err {
	case nil:
		log.Println("[info] success.")
	case context.Canceled:
		log.Panicln("[info] signal catch.")
	default:
		log.Fatalf("[error] %v\n", err)
	}
}
