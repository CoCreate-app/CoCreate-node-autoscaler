/**
 * Copyright (c) 2020 CoCreate LLC
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package provisioner

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/go-logr/logr"
	"github.com/rancher/norman/clientbase"
	managementClient "github.com/rancher/types/client/management/v3"
	"k8s.io/klog/v2"
)

// InternalConfig is a config struct used internal
type InternalConfig struct {
	// RancherURL is URL of target Rancher
	RancherURL string
	// RancherToken is used to access Rancher
	RancherToken string
	// RancherNodePoolID is the ID of node pool which is manipulated
	RancherNodePoolID string
	// RancherCA is used to verify Rancher server
	RancherCA string
}

type provisionerRancherNodePool struct {
	rancherURL        string
	rancherToken      string
	rancherNodePoolID string
	rancherCA         string

	logger logr.Logger

	// management client used to connect to Rancher
	rancherClient *managementClient.Client
}

// NewProvisionerRancherNodePool creates a provisionerRancherNodePool
func NewProvisionerRancherNodePool(cfg InternalConfig) (Provisioner, error) {
	if cfg.RancherNodePoolID == "" {
		return nil, fmt.Errorf("rancher node pool ID must be set to use ranchernodepool provisioner")
	}

	p := &provisionerRancherNodePool{
		rancherURL:        cfg.RancherURL,
		rancherToken:      cfg.RancherToken,
		rancherNodePoolID: cfg.RancherNodePoolID,
		rancherCA:         cfg.RancherCA,
		logger: logger.WithValues("provisioner", ProvisionerRancherNodePool,
			"node pool ID", cfg.RancherNodePoolID),
	}

	err := p.createRancherClient()
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (p *provisionerRancherNodePool) createRancherClient() error {
	opts, err := p.createClientOpts()
	if err != nil {
		p.logger.Error(err, "failed to create Rancher client options")
		return err
	}

	mClient, err := managementClient.NewClient(opts)
	if err != nil {
		p.logger.Error(err, "failed to create Rancher client")
		return err
	}

	p.rancherClient = mClient

	return nil
}

func (p *provisionerRancherNodePool) createClientOpts() (*clientbase.ClientOpts, error) {
	serverURL := p.rancherURL

	if !strings.HasSuffix(serverURL, "/v3") {
		serverURL = p.rancherURL + "/v3"
	}

	var opts *clientbase.ClientOpts

	if p.rancherCA != "" {
		b, err := ioutil.ReadFile(p.rancherCA)
		if err != nil {
			p.logger.Error(err, "failed to read Rancher CA", "Rancher CA", p.rancherCA)
			return nil, err
		}
		opts = &clientbase.ClientOpts{
			URL:      serverURL,
			TokenKey: p.rancherToken,
			CACerts:  string(b),
		}
	} else {
		opts = &clientbase.ClientOpts{
			URL:      serverURL,
			TokenKey: p.rancherToken,
			Insecure: true,
		}
	}

	return opts, nil
}

func (p *provisionerRancherNodePool) Type() ProvisionerT {
	return ProvisionerRancherNodePool
}

func (p *provisionerRancherNodePool) ScaleUp(maxN int) bool {
	defer klog.Flush()
	p.logger.Info("call backend to scale up")

	nodePool, err := p.rancherClient.NodePool.ByID(p.rancherNodePoolID)
	if err != nil {
		p.logger.Error(err, "failed to get Rancher node pool", "node pool ID", p.rancherNodePoolID)
		return false
	}
	p.logger.Info("get node pool info",
		"name", nodePool.Name,
		"node labels", nodePool.NodeLabels,
		"quantity", nodePool.Quantity,
		"display name", nodePool.DisplayName)

	if nodePool.Quantity >= int64(maxN) {
		p.logger.Info("maximum allowed number of nodes reached or exceeded, ignore scaling up",
			"node pool ID", nodePool.ID, "number of existing nodes", nodePool.Quantity)
		return true
	}

	ret := nodePool.Quantity + 1
	go p.rancherClient.NodePool.Update(nodePool, map[string]int64{"quantity": ret})
	return false
}

func (p *provisionerRancherNodePool) ScaleDown(minN int) bool {
	defer klog.Flush()
	p.logger.Info("call backend to scale down")

	nodePool, err := p.rancherClient.NodePool.ByID(p.rancherNodePoolID)
	if err != nil {
		p.logger.Error(err, "failed to get Rancher node pool", "node pool ID", p.rancherNodePoolID)
		return false
	}
	p.logger.Info("get node pool info",
		"name", nodePool.Name,
		"node labels", nodePool.NodeLabels,
		"quantity", nodePool.Quantity,
		"display name", nodePool.DisplayName)

	if nodePool.Quantity <= int64(minN) {
		p.logger.Info("existing number of nodes equals or is below minimum number, ignore scaling down",
			"node pool ID", nodePool.ID, "number of existing nodes", nodePool.Quantity)
		return true
	}

	ret := nodePool.Quantity - 1
	if ret > 0 {
		go p.rancherClient.NodePool.Update(nodePool, map[string]int64{"quantity": ret})
	}
	return false
}
