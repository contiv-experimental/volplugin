package docker

// VolumeCreateRequest is taken from struct Request in https://github.com/calavera/docker-volume-api/blob/master/api.go#L27
type VolumeCreateRequest struct {
	Name string
	Opts map[string]string
}

// Response is taken from struct Response in https://github.com/calavera/docker-volume-api/blob/master/api.go#L33
type Response struct {
	Mountpoint string
	Err        string
}

// Volume represents the docker 'Volume' entity used in get and list
type Volume struct {
	Name       string
	Mountpoint string
}

// VolumeGetRequest is taken from this struct in https://github.com/docker/docker/blob/master/volume/drivers/proxy.go#L187
type VolumeGetRequest struct {
	Name string
}

// VolumeGetResponse is taken from struct volumeDriverProxyGetResponse in https://github.com/docker/docker/blob/master/volume/drivers/proxy.go#L191
type VolumeGetResponse struct {
	Volume Volume
	Err    string
}

// VolumeList is taken from struct volumeDriverProxyListResponse in https://github.com/docker/docker/blob/master/volume/drivers/proxy.go#L163
type VolumeList struct {
	Volumes []Volume
	Err     string
}
