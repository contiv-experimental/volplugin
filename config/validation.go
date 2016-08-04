package config

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

// ValidateJSON validates the given runtime against its defined schema
func (cfg *RuntimeOptions) ValidateJSON() error {
	schema := gojson.NewStringLoader(RuntimeSchema)
	doc := gojson.NewGoLoader(cfg)

	if result, err := gojson.Validate(schema, doc); err != nil {
		return err
	} else if !result.Valid() {
		return combineErrors(result.Errors())
	}

	return nil
}

// ValidateJSON validates the given policy against its defined schema
func (cfg *Policy) ValidateJSON() error {
	schema := gojson.NewStringLoader(PolicySchema)
	doc := gojson.NewGoLoader(cfg)

	if result, err := gojson.Validate(schema, doc); err != nil {
		return err
	} else if !result.Valid() {
		return combineErrors(result.Errors())
	}

	if err := cfg.RuntimeOptions.ValidateJSON(); err != nil {
		return err
	}

	return nil
}

// ValidateJSON validates the given volume against its defined schema
func (cfg *Volume) ValidateJSON() error {
	schema := gojson.NewStringLoader(VolumeSchema)
	doc := gojson.NewGoLoader(cfg)

	if result, err := gojson.Validate(schema, doc); err != nil {
		return err
	} else if !result.Valid() {
		return combineErrors(result.Errors())
	}

	if err := cfg.RuntimeOptions.ValidateJSON(); err != nil {
		return err
	}

	return nil
}
