package test

import (
	"encoding/json"
	"strings"

	"github.com/contiv/errored"
	"github.com/contiv/volplugin/db"
)

type testEntity struct {
	Name            string
	SomeData        string
	FailsValidation bool
	hooks           *db.Hooks
}

func newTestEntity(name, somedata string) *testEntity {
	return &testEntity{Name: name, SomeData: somedata, hooks: &db.Hooks{}}
}

func (t *testEntity) SetKey(key string) error {
	t.Name = strings.Trim(strings.TrimPrefix(strings.Trim(key, "/"), t.Prefix()), "/")
	return nil
}

func (t *testEntity) Read(b []byte) error {
	return json.Unmarshal(b, t)
}

func (t *testEntity) Write() ([]byte, error) {
	return json.Marshal(t)
}

func (t *testEntity) Path() (string, error) {
	return strings.Join([]string{t.Prefix(), t.Name}, "/"), nil
}

func (t *testEntity) Prefix() string {
	return "test"
}

func (t *testEntity) Validate() error {
	if t.FailsValidation {
		return errored.New("failed validation")
	}

	return nil
}

func (t *testEntity) Copy() db.Entity {
	t2 := *t
	return &t2
}

func (t *testEntity) Hooks() *db.Hooks {
	return t.hooks
}
