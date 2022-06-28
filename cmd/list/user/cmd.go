/*
Copyright (c) 2020 Red Hat, Inc.

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

package user

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/spf13/cobra"

	"github.com/openshift/rosa/pkg/ocm"
	"github.com/openshift/rosa/pkg/rosa"
)

var Cmd = &cobra.Command{
	Use:     "users",
	Aliases: []string{"user"},
	Short:   "List cluster users",
	Long:    "List administrative cluster users.",
	Example: `  # List all users on a cluster named "mycluster"
  rosa list users --cluster=mycluster`,
	Run: run,
}

func init() {
	ocm.AddClusterFlag(Cmd)
}

func run(_ *cobra.Command, _ []string) {
	r := rosa.NewRuntime().WithAWS().WithOCM()
	defer r.Cleanup()

	clusterKey := r.GetClusterKey()

	// Try to find the cluster:
	r.Reporter.Debugf("Loading cluster '%s'", clusterKey)
	cluster, err := r.OCMClient.GetCluster(clusterKey, r.Creator)
	if err != nil {
		r.Reporter.Errorf("Failed to get cluster '%s': %v", clusterKey, err)
		os.Exit(1)
	}

	if cluster.State() != cmv1.ClusterStateReady {
		r.Reporter.Errorf("Cluster '%s' is not yet ready", clusterKey)
		os.Exit(1)
	}

	var clusterAdmins []*cmv1.User
	r.Reporter.Debugf("Loading users for cluster '%s'", clusterKey)
	// Load cluster-admins for this cluster
	clusterAdmins, err = r.OCMClient.GetUsers(cluster.ID(), "cluster-admins")
	if err != nil {
		r.Reporter.Errorf("Failed to get cluster-admins for cluster '%s': %v", clusterKey, err)
		os.Exit(1)
	}
	// Remove cluster-admin user
	for i, user := range clusterAdmins {
		if user.ID() == "cluster-admin" {
			clusterAdmins = append(clusterAdmins[:i], clusterAdmins[i+1:]...)
		}
	}

	// Load dedicated-admins for this cluster
	dedicatedAdmins, err := r.OCMClient.GetUsers(cluster.ID(), "dedicated-admins")
	if err != nil {
		r.Reporter.Errorf("Failed to get dedicated-admins for cluster '%s': %v", clusterKey, err)
		os.Exit(1)
	}

	if len(clusterAdmins) == 0 && len(dedicatedAdmins) == 0 {
		r.Reporter.Warnf("There are no users configured for cluster '%s'", clusterKey)
		os.Exit(1)
	}

	groups := make(map[string][]string)
	for _, user := range clusterAdmins {
		groups[user.ID()] = []string{"cluster-admins"}
	}
	for _, user := range dedicatedAdmins {
		if _, ok := groups[user.ID()]; ok {
			groups[user.ID()] = []string{"cluster-admins", "dedicated-admins"}
		} else {
			groups[user.ID()] = []string{"dedicated-admins"}
		}
	}

	// Create the writer that will be used to print the tabulated results:
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(writer, "ID\t\tGROUPS\n")

	for u, r := range groups {
		fmt.Fprintf(writer, "%s\t\t%s\n", u, strings.Join(r, ", "))
		writer.Flush()
	}
}
