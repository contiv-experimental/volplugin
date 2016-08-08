package entities

import (
	"fmt"
	"strings"

	"github.com/contiv/errored"

	gojson "github.com/xeipuuv/gojsonschema"
)

// Combines array of errors into a single error
func combineErrors(resultErrors []gojson.ResultError) error {
	var errors []string
	for _, err := range resultErrors {
		errors = append(errors, fmt.Sprintf("%s\n", err))
	}
	return errored.New(strings.Join(errors, "\n"))
}
