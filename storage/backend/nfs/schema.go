package nfs

var (
	// DriverOptionsSchema defines json schema for nfs driver options
	DriverOptionsSchema = `{  
    "title":"Driver options validation",
    "type":"object",
    "properties":{  
      "options":{ "type":"string" }
    }
  }`
)
