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

func (t *testEntity) String() string {
	return t.Name
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

type testLock struct {
	owner    string
	reason   string
	name     string
	mayExist bool
}

func newLock(mayExist bool, owner, reason, name string) *testLock {
	return &testLock{owner: owner, reason: reason, name: name, mayExist: mayExist}
}

func (t *testLock) String() string {
	return t.name
}

func (t *testLock) SetKey(key string) error {
	t.name = strings.Trim(strings.TrimPrefix(strings.Trim(key, "/"), t.Prefix()), "/")
	return nil
}

func (t *testLock) Read(b []byte) error {
	return json.Unmarshal(b, t)
}

func (t *testLock) Write() ([]byte, error) {
	return json.Marshal(t)
}

func (t *testLock) Path() (string, error) {
	return strings.Join([]string{t.Prefix(), t.name}, "/"), nil
}

func (t *testLock) Prefix() string {
	return "test"
}

func (t *testLock) Validate() error {
	return nil
}

func (t *testLock) Copy() db.Entity {
	t2 := *t
	return &t2
}

func (t *testLock) Hooks() *db.Hooks {
	return &db.Hooks{}
}

func (t *testLock) Owner() string {
	return t.owner
}

func (t *testLock) Reason() string {
	return t.reason
}
