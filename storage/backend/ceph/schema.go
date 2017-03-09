package ceph

var (
	// DriverOptionsSchema defines json schema for ceph driver options
	DriverOptionsSchema = `{
    "title": "Driver options validation",
    "type": "object",
    "properties": {
      "pool": { "type": "string", "minLen": 1}
    }
  }`

	// GlusterSchema TODO
	GlusterSchema = `{
    "title": "GlusterFS driverOpts validation",
    "type": "object",
    "properties": {
      "replica": { "type": "number", "minimum": 1 },
      "transport": { "type": "string", "enum": [ "tcp", "rdma" ] },
      "stripe": { "type": "number", "minimum": 1 },
      "bricks": {
        "type": "object",
        "patternProperties": {
          ".{1,}": { "type": "string" }
        }
      }
    }, 
    "required": [ "bricks" ] 
  }`
)
