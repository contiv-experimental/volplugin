package client

import "golang.org/x/net/context"

var initPaths = []string{
	"policies",
	"policy-archives",
}

// Init initializes the data store. It does this by creating directories (and
// subdirectories) as necessary for our keyspaces.
func Init(cli Client) {
	for _, str := range initPaths {
		path, _ := cli.Path().Append(str) // deliberately skipping the error because we have input from constants only
		cli.Set(context.Background(), path, nil, &SetOptions{Dir: true})
	}
}
