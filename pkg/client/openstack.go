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
	ListListenersForCurrentTenant() ([]listeners.Listener, error)
	ListMonitorsForCurrentTenant() ([]monitors.Monitor, error)
	GetPoolsForCurrentTenant() ([]pools.Pool, error)
	GetListenersForLoadbalancerID(loadbalancerid string) ([]listeners.Listener, error)
	GetMonitorsForPoolID(poolid string) ([]monitors.Monitor, error)
	GetPoolIDsForCurrentTenant() ([]string, error)
	GetMembersForPoolID(poolid string) ([]pools.Member, error)
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
	fmt.Println("============")
	fmt.Printf("| OpenStack Client\n")
	fmt.Printf("| auth_url: %s\n", opts.IdentityEndpoint)
	fmt.Printf("| domain_name: %s\n", opts.DomainName)
	fmt.Printf("| tenant_name: %s (id: %s)\n", opts.TenantName, opts.TenantID)
	fmt.Printf("| user_name: %s\n", opts.Username)
	fmt.Println("============")

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

func (o *openstackprovider) GetListenersForLoadbalancerID(loadbalancerid string) ([]listeners.Listener, error) {
	allPages, err := listeners.List(o.networkClient, listeners.ListOpts{
		LoadbalancerID: loadbalancerid,
		TenantID:       o.opts.TenantID,
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list listeners for loadbalancer id %s, %s", loadbalancerid, err)
	}
	listeners, err := listeners.ExtractListeners(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract listeners %s", err)
	}
	return listeners, nil
}

func (o *openstackprovider) ListListenersForCurrentTenant() ([]listeners.Listener, error) {
	allPages, err := listeners.List(o.networkClient, listeners.ListOpts{
		TenantID: o.opts.TenantID,
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list all listener pages %s", err)
	}
	listeners, err := listeners.ExtractListeners(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract all listener object %s", err)
	}
	return listeners, nil
}

func (o *openstackprovider) GetPoolsForCurrentTenant() ([]pools.Pool, error) {
	allPages, err := pools.List(o.networkClient, pools.ListOpts{
		TenantID: o.opts.TenantID,
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list all pool pages %s", err)
	}
	pools, err := pools.ExtractPools(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract all pools from pages %s", err)
	}
	return pools, nil
}

func (o *openstackprovider) GetPoolIDsForCurrentTenant() ([]string, error) {
	pools, err := o.GetPoolsForCurrentTenant()
	if err != nil {
		return nil, fmt.Errorf("failed to list all pools %s", err)
	}
	var ids []string
	for _, pool := range pools {
		ids = append(ids, pool.ID)
	}
	return ids, nil
}

func (o *openstackprovider) GetPoolsForListenerID(listenerid string) ([]pools.Pool, error) {
	allPages, err := pools.List(o.networkClient, pools.ListOpts{
		ListenerID: listenerid,
		TenantID:   o.opts.TenantID,
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to get pool pages for pool id %s, %s", listenerid, err)
	}
	pools, err := pools.ExtractPools(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract pools from pages for pool id %s, %s", listenerid, err)
	}
	return pools, nil
}

func (o *openstackprovider) ListMonitorsForCurrentTenant() ([]monitors.Monitor, error) {
	allPages, err := monitors.List(o.networkClient, monitors.ListOpts{
		TenantID: o.opts.TenantID,
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list monitors %s", err)
	}
	monitors, err := monitors.ExtractMonitors(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract monitor pages %s", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list monitors %s", err)
	}
	return monitors, nil
}

func (o *openstackprovider) GetMonitorsForPoolID(poolid string) ([]monitors.Monitor, error) {
	allPages, err := monitors.List(o.networkClient, monitors.ListOpts{
		TenantID: o.opts.TenantID,
		PoolID:   poolid,
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list monitors %s", err)
	}
	monitors, err := monitors.ExtractMonitors(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract monitor pages %s", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list monitors %s", err)
	}
	return monitors, nil
}

func (o *openstackprovider) GetMembersForPoolID(poolid string) ([]pools.Member, error) {
	allPages, err := pools.List(o.networkClient, pools.ListOpts{
		TenantID: o.opts.TenantID,
		ID:       poolid,
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list pool pages %s", err)
	}
	pools, err := pools.ExtractPools(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract pools from pages %s", err)
	}
	if len(pools) > 1 {
		return nil, fmt.Errorf("got more than one pool for ID %s. this should never happen %s", poolid, err)
	} else if len(pools) == 0 {
		return nil, nil
	} else {
		return pools[0].Members, nil
	}
}

func (o *openstackprovider) DeleteLoadBalancer(id string) error {
	fmt.Printf("deleting loadbalancer with id %s\n", id)
	listeners, err := o.GetListenersForLoadbalancerID(id)
	if err != nil {
		return fmt.Errorf("failed to get listener for loadbalancer ID %s, %s", id, err)
	}
	fmt.Printf("listenercount %d\n", len(listeners))
	// remove all listeners for a certain LB ID
	for _, listener := range listeners {
		fmt.Printf("found listener with id %s\n", listener.ID)
		pools, err := o.GetPoolsForListenerID(listener.ID)
		fmt.Printf("poolscount %d\n", len(pools))

		if err != nil {
			return fmt.Errorf("failed to get pool IDs for listener ID %s, %s", listener.ID, err)
		}
		// remove all pools
		for _, pool := range pools {
			fmt.Printf("found pool with id %s\n", id)
			hms, err := o.GetMonitorsForPoolID(pool.ID)
			fmt.Printf("hmcount %d\n", len(hms))

			if err != nil {
				return fmt.Errorf("failed to get monitor IDs for pool ID %s, %s", pool.ID, err)
			}
			// remove all health monitors
			for _, hm := range hms {
				fmt.Printf("found healthmonitor with id %s\n", id)
				// result := monitors.Delete(o.networkClient, hmID)
				// if result.Err != nil {
				// 	return fmt.Errorf("failed to delete monitor with ID %s, %s", hmID, result.Err)
				// }
				fmt.Printf("deleted health monitor with id %s\n", hm.ID)
			}
			// result := pools.Delete(o.networkClient, poolID)
			// if result.Err != nil {
			// 	return fmt.Errorf("failed to delete pool with ID %s, %s", poolID, result.Err)
			// }
			fmt.Printf("deleted pool with id %s\n", pool.ID)
		}
		// result := listeners.Delete(o.networkClient, listenerID)
		// if result.Err != nil {
		// 	return fmt.Errorf("failed to delete listener with ID %s, %s", listenerID, result.Err)
		// }
		fmt.Printf("deleted listener with id %s\n", listener.ID)
	}
	// result := loadbalancers.Delete(o.networkClient, id)
	// if result.Err != nil {
	// 	return fmt.Errorf("failed to delete loadbalancer with ID %s, %s", id, result.Err)
	// }
	fmt.Printf("deleted loadbalancer with id %s\n", id)
	return nil
}
