// Copyright © 2018 NAME HERE <andreas.fritzler@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"fmt"
	"os"

	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/lbaas_v2/listeners"

	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/lbaas_v2/pools"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/lbaas_v2/loadbalancers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/lbaas_v2/monitors"
)

type OpenStackProvider interface {
	ListLBaaSIDs() ([]string, error)
	ListLBaaS() ([]loadbalancers.LoadBalancer, error)
	ListListenerIDsForCurrentTenant() ([]string, error)
	ListMonitorIDsForCurrentTenant() ([]string, error)
	GetMonitorIDsForPoolID(poolid string) ([]string, error)
	GetPoolIDsForListenerID(listenerid string) ([]string, error)
	GetListenerIDsForLoadbalancerID(loadbalancerid string) ([]string, error)
	GetPoolIDsForCurrentTenant() ([]string, error)
	DeleteLoadBalancer(id string) error
}

type openstackprovider struct {
	opts          *gophercloud.AuthOptions
	provider      *gophercloud.ProviderClient
	networkClient *gophercloud.ServiceClient
}

func NewOpenStackProvider() (OpenStackProvider, error) {
	opts, err := openstack.AuthOptionsFromEnv()
	opts.DomainName = os.Getenv("OS_USER_DOMAIN_NAME")
	fmt.Printf("Creating client for: auth_url: %s, domain_name: %s, tenant_name: %s (id: %s), user_name: %s\n",
		opts.IdentityEndpoint, opts.DomainName, opts.TenantName, opts.TenantID, opts.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth opts from environment %s", err)
	}
	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated client %s", err)
	}
	networkClient, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get network client %s", err)
	}
	return &openstackprovider{opts: &opts, provider: provider, networkClient: networkClient}, nil
}

func (o *openstackprovider) ListLBaaS() ([]loadbalancers.LoadBalancer, error) {
	client, err := openstack.NewNetworkV2(o.provider, gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create neutron client %s", err)
	}
	allPages, err := loadbalancers.List(client, loadbalancers.ListOpts{
		TenantID: o.opts.TenantID,
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list all loadbalancers %s", err)
	}
	actual, err := loadbalancers.ExtractLoadBalancers(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to list all external loadbalancers %s", err)
	}
	return actual, nil
}

func (o *openstackprovider) ListLBaaSIDs() ([]string, error) {
	lbList, err := o.ListLBaaS()
	if err != nil {
		return nil, fmt.Errorf("failed to list all loadbalancers %s", err)
	}
	ids := make([]string, len(lbList))
	for idx, lb := range lbList {
		ids[idx] = lb.ID
	}
	return ids, nil
}

func (o *openstackprovider) GetListenerIDsForLoadbalancerID(loadbalancerid string) ([]string, error) {
	allPages, err := listeners.List(o.networkClient, listeners.ListOpts{
		LoadbalancerID: loadbalancerid,
		TenantID:       o.opts.TenantID,
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list listeners for loadbalancer id %s", err)
	}
	listeners, err := listeners.ExtractListeners(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract listeners %s", err)
	}
	var ids []string
	for _, listener := range listeners {
		ids = append(ids, listener.ID)
	}
	return ids, nil
}

func (o *openstackprovider) ListListenerIDsForCurrentTenant() ([]string, error) {
	lbList, err := o.ListLBaaS()
	if err != nil {
		return nil, fmt.Errorf("failed to list all loadbalancers %s", err)
	}
	var ids []string
	for _, lb := range lbList {
		listeners := lb.Listeners
		for _, listener := range listeners {
			ids = append(ids, listener.ID)
		}
	}
	return ids, nil
}

func (o *openstackprovider) GetPoolIDsForCurrentTenant() ([]string, error) {
	listeners, err := o.ListListenerIDsForCurrentTenant()
	if err != nil {
		return nil, fmt.Errorf("failed to list all loadbalancers %s", err)
	}
	var ids []string
	for _, listenerID := range listeners {
		poolIDs, err := o.GetPoolIDsForListenerID(listenerID)
		if err != nil {
			return nil, fmt.Errorf("failed to list all pool ids %s", err)
		}
		ids = append(poolIDs)
	}
	return ids, nil
}

func (o *openstackprovider) GetPoolIDsForListenerID(listenerID string) ([]string, error) {
	allPages, err := pools.List(o.networkClient, pools.ListOpts{
		ListenerID: listenerID,
		TenantID:   o.opts.TenantID,
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to extract pool pages %s", err)
	}
	pools, err := pools.ExtractPools(allPages)
	var ids []string
	for _, pool := range pools {
		ids = append(ids, pool.ID)
	}
	return ids, nil
}

// get all monitor IDs for current tenant
func (o *openstackprovider) ListMonitorIDsForCurrentTenant() ([]string, error) {
	monitors, err := o.extractMonitoringPages(monitors.ListOpts{
		TenantID: o.opts.TenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to extract monitor pages %s", err)
	}
	var ids []string
	for _, monitor := range monitors {
		ids = append(ids, monitor.ID)
	}
	return ids, nil
}

func (o *openstackprovider) GetMonitorIDsForPoolID(poolid string) ([]string, error) {
	monitors, err := o.extractMonitoringPages(monitors.ListOpts{
		PoolID:   poolid,
		TenantID: o.opts.TenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get monitor %s", err)
	}
	var ids []string
	for _, monitor := range monitors {
		ids = append(ids, monitor.ID)
	}
	return ids, nil
}

func (o *openstackprovider) DeleteLoadBalancer(id string) error {
	fmt.Printf("deleted %s", id)
	listenersIDs, err := o.GetListenerIDsForLoadbalancerID(id)
	if err != nil {
		return fmt.Errorf("failed to get listener for loadbalancer ID %s, %s", id, err)
	}
	// remove all listeners for a certain LB ID
	for _, listenerID := range listenersIDs {
		poolIds, err := o.GetPoolIDsForListenerID(listenerID)
		if err != nil {
			return fmt.Errorf("failed to get pool IDs for listener ID %s, %s", listenerID, err)
		}
		// remove all pools
		for _, poolID := range poolIds {
			hmIDs, err := o.GetMonitorIDsForPoolID(poolID)
			if err != nil {
				return fmt.Errorf("failed to get monitor IDs for pool ID %s, %s", poolID, err)
			}
			// remove all health monitors
			for _, hmID := range hmIDs {
				result := monitors.Delete(o.networkClient, hmID)
				if result.Err != nil {
					return fmt.Errorf("failed to delete monitor with ID %s, %s", hmID, result.Err)
				}
				fmt.Printf("deleted health monitor with id %s\n", hmID)
			}
			result := pools.Delete(o.networkClient, poolID)
			if result.Err != nil {
				return fmt.Errorf("failed to delete pool with ID %s, %s", poolID, result.Err)
			}
			fmt.Printf("deleted pool with id %s\n", poolID)
		}
		result := listeners.Delete(o.networkClient, listenerID)
		if result.Err != nil {
			return fmt.Errorf("failed to delete listener with ID %s, %s", listenerID, result.Err)
		}
		fmt.Printf("deleted listener with id %s\n", listenerID)
	}
	result := loadbalancers.Delete(o.networkClient, id)
	if result.Err != nil {
		return fmt.Errorf("failed to delete loadbalancer with ID %s, %s", id, result.Err)
	}
	fmt.Printf("deleted loadbalancer with id %s\n", id)
	return nil
}

func (o *openstackprovider) extractMonitoringPages(listOpts monitors.ListOpts) ([]monitors.Monitor, error) {
	allPages, err := monitors.List(o.networkClient, listOpts).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list monitors %s", err)
	}
	actual, err := monitors.ExtractMonitors(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract monitor pages %s", err)
	}
	return actual, nil
}
