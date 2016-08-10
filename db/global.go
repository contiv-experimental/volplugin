package db

// NewGlobal constructs a new global object.
func NewGlobal() *Global {
	return &Global{}
}

// Path returns the path to the global configuration
func (g *Global) Path() (string, error) {
	return rootGlobal, nil
}

// Prefix returns the path to the global configuration
func (g *Global) Prefix() string {
	return ""
}

// Validate validates the global configuration.
func (g *Global) Validate() error {
	if g.MountPath == "" {
		g.MountPath = DefaultMountPath
	}

	if g.TTL < TTLFixBase {
		g.TTL = DefaultGlobalTTL
	}

	if g.Timeout < TimeoutFixBase {
		g.Timeout = DefaultTimeout
	}

	return nil
}

// Hooks returns an empty hooks set.
func (g *Global) Hooks() *Hooks {
	return &Hooks{}
}

// Copy returns a copy of this Global.
func (g *Global) Copy() Entity {
	g2 := *g
	return &g2
}

// Published returns a copy of the current global with the parameters adjusted
// to fit the published representation. To see the internal/system/canonical
// version, please see Canonical() below.
//
// It is very important that you do not run this function multiple times
// against the same data set. It will adjust the parameters twice.
func (g *Global) Published() *Global {
	newGlobal := *g

	newGlobal.TTL /= TTLFixBase
	newGlobal.Timeout /= TimeoutFixBase

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

	if g.TTL < TTLFixBase {
		newGlobal.TTL *= TTLFixBase
	}

	if g.Timeout < TimeoutFixBase {
		newGlobal.Timeout *= TimeoutFixBase
	}

	return &newGlobal
}
