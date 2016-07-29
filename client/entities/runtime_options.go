package entities

import (
	gojson "github.com/xeipuuv/gojsonschema"
)

// ValidateJSON validates the given runtime against its defined schema
func (ro *RuntimeOptions) ValidateJSON() error {
	schema := gojson.NewStringLoader(RuntimeSchema)
	doc := gojson.NewGoLoader(ro)

	if result, err := gojson.Validate(schema, doc); err != nil {
		return err
	} else if !result.Valid() {
		return combineErrors(result.Errors())
	}

	return nil
}
