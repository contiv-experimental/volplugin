package entities

import (
	"encoding/json"
	"time"

	"golang.org/x/net/context"

	"github.com/contiv/volplugin/client"
	"github.com/contiv/volplugin/errors"
)

const (
	// DefaultGlobalTTL is the TTL used when no TTL exists.
	DefaultGlobalTTL = 30 * time.Second
	// DefaultTimeout is the standard command timeout when none is provided.
	DefaultTimeout = 10 * time.Minute

	timeoutFixBase   = time.Minute
	ttlFixBase       = time.Second
	defaultMountPath = "/mnt/ceph"

	globalPathName = "global-config"
)

// NewGlobalFromJSON transforms json into a global. Conforms to
// client.Entity interface.
func NewGlobalFromJSON(content []byte) (*Global, error) {
	global := NewGlobal()

	if err := json.Unmarshal(content, global); err != nil {
		return nil, err
	}

	return global.setEmpty(), nil
}

// NewGlobal returns global config with preset defaults. Conforms to
// client.Entity interface.
func NewGlobal() *Global {
	return &Global{
		TTL:       DefaultGlobalTTL,
		MountPath: defaultMountPath,
		Timeout:   DefaultTimeout,
	}
}

// Path returns the path of the global configuration
func (g *Global) Path(p *client.Pather) (string, error) {
	inter, err := p.Append(globalPathName)
	if err != nil {
		return "", err
	}

	return inter.String(), nil
}

// Payload returns the global configuration in JSON form.
func (g *Global) Payload() ([]byte, error) {
	content, err := json.Marshal(g)
	if err != nil {
		return nil, errors.MarshalGlobal.Combine(err)
	}

	return content, nil
}

// Published returns a copy of the current global with the parameters adjusted
// to fit the published representation. To see the internal/system/canonical
// version, please see Canonical() below.
//
// It is very important that you do not run this function multiple times
// against the same data set. It will adjust the parameters twice.
func (g *Global) Published() *Global {
	newGlobal := *g

	newGlobal.TTL /= ttlFixBase
	newGlobal.Timeout /= timeoutFixBase

	return &newGlobal
}

// Canonical returns a copy of the current global with the parameters adjusted
// to fit the internal (or canonical) representation. To see the published
// version, see Published() above.
//
// It is very important that you do not run this function multiple times
// against the same data set. It will adjust the parameters twice.
func (g *Global) Canonical() *Global {
	newGlobal := *g

	if g.TTL < ttlFixBase {
		newGlobal.TTL *= ttlFixBase
	}

	if g.Timeout < timeoutFixBase {
		newGlobal.Timeout *= timeoutFixBase
	}

	return &newGlobal
}

// setEmpty sets any emptied parameters. This is typically used during the
// creation of the object accepted from user input.
func (g *Global) setEmpty() *Global {
	newGlobal := *g

	if newGlobal.Timeout == 0 {
		newGlobal.Timeout = DefaultTimeout
	}

	if g.TTL == 0 {
		newGlobal.TTL = DefaultGlobalTTL
	}

	if g.MountPath == "" {
		newGlobal.MountPath = defaultMountPath
	}

	return &newGlobal
}

// NewGlobalClient yield a client to fetching Global entities. Conforms to client.EntityClient interface.
func NewGlobalClient(client client.Client) *GlobalClient {
	path, _ := client.Path().Replace(globalPathName) // only the prefix is variable.
	return &GlobalClient{client: client, path: path}
}

// Delete the global. Name is ignored.
func (gc *GlobalClient) Delete(name string) error {
	return gc.client.Delete(context.Background(), gc.path, nil)
}

// Get the global configuration. Currently, the string argument is ignored
// because there is only one of these.
func (gc *GlobalClient) Get(name string) (*Global, error) {
	resp, err := gc.client.Get(context.Background(), gc.path, nil)
	if err != nil {
		return nil, errors.GetGlobal.Combine(err)
	}

	return NewGlobalFromJSON(resp.Value)
}

func (gc *GlobalClient) processWatch(node *client.Node, channel chan interface{}) error {
	global, err := NewGlobalFromJSON(node.Value)
	if err != nil {
		return err
	}

	channel <- global
	return nil
}

// Watch the global. Path is ignored.
func (gc *GlobalClient) Watch(name string) (chan interface{}, chan error, error) {
	channel := make(chan interface{}, 1)
	// stopchan is not taken here because it is stored and can be
	// used via WatchStop().
	// TODO evaluate whether returning the stop channel is necessary.
	_, errChan := gc.client.Watch(
		gc.path,
		channel,
		false,
		gc.processWatch,
	)
	return channel, errChan, nil
}

// WatchStop stops an existing watch on a path.
func (gc *GlobalClient) WatchStop(name string) error {
	gc.client.WatchStop(gc.path)
	return nil
}

// Publish commits the global to the database.
func (gc *GlobalClient) Publish(g *Global) error {
	value, err := g.Canonical().Payload()
	if err != nil {
		return err
	}

	if err := gc.client.Set(context.Background(), gc.path, value, &client.SetOptions{Exist: client.PrevIgnore}); err != nil {
		return errors.EtcdToErrored(err)
	}

	return nil
}

// Collection yields nil, as there are no collections for globals.
func (gc *GlobalClient) Collection() client.EntityCollection {
	return nil
}

// Path returns the path to to the volplugin root, since it lives directly off it as a file.
func (gc *GlobalClient) Path() *client.Pather {
	return gc.client.Path()
}
