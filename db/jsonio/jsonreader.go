package jsonio

import (
	"encoding/json"

	"github.com/contiv/volplugin/db"
	"github.com/contiv/volplugin/errors"
)

// Read into the entity.
func Read(obj db.Entity, b []byte) error {
	if err := json.Unmarshal(b, obj); err != nil {
		return errors.UnmarshalPolicy.Combine(err)
	}

	return nil
}

// Write turns the entity into JSON for writing.
func Write(obj db.Entity) ([]byte, error) {
	content, err := json.Marshal(obj)
	if err != nil {
		return nil, errors.MarshalPolicy.Combine(err)
	}

	return content, nil
}
