package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/codegangsta/cli"
)

func start(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, errors.New("Config file required."))
	}

	configFile := ctx.Args()[0]

	content, err := ioutil.ReadFile(configFile)
	if err != nil {
		errExit(ctx, err)
	}

	var config Config

	if err := json.Unmarshal(content, &config); err != nil {
		errExit(ctx, err)
	}

	if err := config.validate(); err != nil {
		errExit(ctx, err)
	}

	daemon(config)
}

func main() {
	basePath := path.Base(os.Args[0])

	app := cli.NewApp()
	app.Version = "0.0.1"
	app.Usage = fmt.Sprintf("Control many volplugins: %s [config file]", basePath)
	app.Name = basePath
	app.Action = start
	app.Run(os.Args)
}
