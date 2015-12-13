/***
Copyright 2014 Cisco Systems Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vagrantssh

import (
	"fmt"
	"io/ioutil"
	"net"

	"golang.org/x/crypto/ssh"
)

// VagrantNode implements a node in vagrant testbed
type VagrantNode struct {
	Name      string
	primaryIP net.IP
	port      string
	config    *ssh.ClientConfig
}

// NewVagrantNode intializes a node in vagrant testbed
func NewVagrantNode(name, port, privKeyFile string) (*VagrantNode, error) {
	var (
		err        error
		signer     ssh.Signer
		privateKey []byte
	)

	if privateKey, err = ioutil.ReadFile(privKeyFile); err != nil {
		return nil, err
	}

	if signer, err = ssh.ParsePrivateKey(privateKey); err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: "vagrant",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}

	return &VagrantNode{Name: name, port: port, config: config}, nil
}

func (n *VagrantNode) dial() (*ssh.Client, error) {
	client, err := ssh.Dial("tcp", fmt.Sprintf("127.0.0.1:%s", n.port), n.config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// Cleanup does nothing for vagrant
func (n *VagrantNode) Cleanup() {}

func newCmdStrWithSource(cmd string) string {
	// we need to source the environment manually as the ssh package client
	// doesn't do it automatically (I guess something to do with non interative
	// mode)
	return fmt.Sprintf("bash -lc '%s'", cmd)
}

func (n *VagrantNode) GetClientAndSession() (*ssh.Client, *ssh.Session, error) {
	client, err := n.dial()
	if err != nil {
		return nil, nil, err
	}

	s, err := client.NewSession()
	if err != nil {
		return nil, nil, err
	}

	return client, s, nil
}

// RunCommand runs a shell command in a vagrant node and returns it's exit status
func (n *VagrantNode) RunCommand(cmd string) error {
	client, s, err := n.GetClientAndSession()
	if err != nil {
		return err
	}

	defer client.Close()
	defer s.Close()

	return s.Run(newCmdStrWithSource(cmd))
}

// RunCommandWithOutput runs a shell command in a vagrant node and returns it's
// exit status and output
func (n *VagrantNode) RunCommandWithOutput(cmd string) (string, error) {
	client, s, err := n.GetClientAndSession()
	if err != nil {
		return "", err
	}

	defer client.Close()
	defer s.Close()

	output, err := s.CombinedOutput(newCmdStrWithSource(cmd))
	return string(output), err
}

// RunCommandBackground runs a background command in a vagrant node.
func (n *VagrantNode) RunCommandBackground(cmd string) error {
	// XXX we leak a connection here so we can keep the session alive. While this
	// is less than ideal it allows us to "fire and forget" from our perspective,
	// and give system tests the ability to manage the background processes themselves.
	_, s, err := n.GetClientAndSession()
	if err != nil {
		return err
	}

	// start and forget about the command as user asked to run in background.
	// The limitation is we/ won't know if it fails though. Not a worry right
	// now as the test will fail anyways, but might be good to find a better way.
	return s.Start(newCmdStrWithSource(cmd))
}

// GetName returns vagrant node's name
func (n *VagrantNode) GetName() string {
	return n.Name
}
