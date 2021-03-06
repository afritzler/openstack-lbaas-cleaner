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

package cmd

import (
	"fmt"

	"github.com/afritzler/oli/pkg/client"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
func deleteCmd() *cobra.Command {
	var noDryRun bool
	c := &cobra.Command{
		Use:   "delete <LoadBalancerID>",
		Short: "Delete a LoadBalancer + everything attached",
		Long:  `Delete a LoadBalancer + everything attached.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			osClient, err := client.NewOpenStackProvider(client.Config{
				DryRun: !noDryRun,
			})
			if err != nil {
				panic(fmt.Errorf("failed to create os client %s", err))
			}
			err = osClient.DeleteLoadBalancer(args[0])
			if err != nil {
				panic(fmt.Errorf("failed to delete loadbalancer %s", err))
			}
		},
	}
	c.Flags().BoolVar(&noDryRun, "no-dry-run", false, "The real deal!")
	return c
}

func init() {
	rootCmd.AddCommand(deleteCmd())
}
