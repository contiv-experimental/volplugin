package config

var (
	// RuntimeSchema defines json schema for runtime configuration
	RuntimeSchema = `{
		"title": "Runtime config validation",
		"type": "object",
		"oneOf": [ {
			"properties": {
				"snapshots": { "enum": [ true ] },
				"snapshot": {
					"type": "object",
					"properties": {
						"frequency": { "type": "string", "pattern": "^[0-9]+.$", "minLength": 1 },
						"keep": { "type": "number", "minimum": 1 }
					},
					"required": [ "frequency", "keep" ]
				}
			}
			},
			{ "properties": { "snapshots": { "enum": [ false ] } } }
		]
	}`

	// PolicySchema defines the json schema for policy
	PolicySchema = `{
		"title": "Policy config validation",
		"type": "object",
		"properties": {
			"name": {"type": "string", "minLength": 1},
			"backends": {
				"type": "object",
				"properties": {
					"mount": { "type": "string", "minLength": 1, "enum": [ "ceph", "nfs" ] },
					"crud": { "type": "string", "enum": [ "ceph", "" ] },
					"snapshot": { "type": "string", "enum": [ "ceph", "" ] }
				},
				"required": [ "mount" ]
			}, 
			"backend": { "enum": [ "ceph", "nfs" ] },
			"rate-limit": {
				"type": "object",
				"write-bps": { "type": "number", "minimum": 1},
				"read-bps": { "type": "number", "minimum": 1}
			}
		},
		"anyOf": [
			{ "required": [ "backend" ] },
			{ "required": [ "backends" ] }
		], 
		"required": [ "name" ]
	}`

	//VolumeSchema defines the json schema for volume
	VolumeSchema = `{
		"title": "Volume config validation",
		"type": "object",
		"properties": {
			"name": { "type": "string", "minLength": 1 },
			"policy": { "type": "string", "minLength": 1 },
			"backends": {
				"type": "object",
				"properties": {
					"mount": { "type": "string", "minLength": 1, "enum": [ "ceph", "nfs" ] },
					"crud": { "type": "string", "enum": [ "ceph", "" ] },
					"snapshot": { "type": "string", "enum": [ "ceph", "" ] }
				},
				"required": [ "mount" ]
			}
		},
		"required": [ "name", "policy", "backends" ]
	}`
)
