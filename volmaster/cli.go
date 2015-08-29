package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

func start(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, errors.New("Config file required."))
	}

	if ctx.Bool("debug") {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging enabled")
	}

	configFile := ctx.Args()[0]

	content, err := ioutil.ReadFile(configFile)
	if err != nil {
		errExit(ctx, err)
	}

	var config config

	if err := json.Unmarshal(content, &config); err != nil {
		errExit(ctx, err)
	}

	if err := config.validate(); err != nil {
		errExit(ctx, err)
	}

	daemon(config, ctx.Bool("debug"), ctx.String("listen"))
}

func main() {
	basePath := path.Base(os.Args[0])

	app := cli.NewApp()
	app.Version = "0.0.1"
	app.Usage = fmt.Sprintf("Control many volplugins: %s [config file]", basePath)
	app.Name = basePath
	app.Action = start
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "turn on debugging",
			EnvVar: "DEBUG",
		},
		cli.StringFlag{
			Name:   "listen",
			Usage:  "listen address for volmaster",
			EnvVar: "LISTEN",
			Value:  ":8080",
		},
	}
	app.Run(os.Args)
}
