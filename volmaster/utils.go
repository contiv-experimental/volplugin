package main

import (
	"fmt"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

func errExit(ctx *cli.Context, err error) {
	fmt.Printf("\nError: %v\n\n", err)
	cli.ShowAppHelp(ctx)
	os.Exit(1)
}

func httpError(w http.ResponseWriter, message string, err error) {
	fullError := fmt.Sprintf("%s %v", message, err)

	log.Warnf("Returning HTTP error handling plugin negotiation: %s", fullError)
	http.Error(w, fullError, http.StatusInternalServerError)
}
