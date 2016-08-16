package db

import (
	"fmt"
	"strings"

	"github.com/contiv/errored"
	gojson "github.com/xeipuuv/gojsonschema"
)

func validateJSON(schema string, obj Entity) error {
	schemaObj := gojson.NewStringLoader(schema)
	doc := gojson.NewGoLoader(obj)

	if result, err := gojson.Validate(schemaObj, doc); err != nil {
		return err
	} else if !result.Valid() {
		var errors []string
		for _, err := range result.Errors() {
			errors = append(errors, fmt.Sprintf("%s\n", err))
		}
		return errored.New(strings.Join(errors, "\n"))
	}

	return nil
}
