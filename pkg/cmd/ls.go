package cmd

import (
	"context"
	"fmt"
	"strings"

	cmdutil "github.com/cloud-native-application/rudrx/pkg/cmd/util"
	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewAppsCommand(f cmdutil.Factory, c client.Client, ioStreams cmdutil.IOStreams) *cobra.Command {
	ctx := context.Background()
	cmd := &cobra.Command{
		Use:                   "ls",
		DisableFlagsInUseLine: true,
		Short:                 "List applications",
		Long:                  "List applications with workloads, traits, status and created time",
		Example:               `rudr ls`,
		Run: func(cmd *cobra.Command, args []string) {
			//appName := cmd.Flag("app").Value.String()
			appName := cmd.Flag("app").Value.String()
			namespace := cmd.Flag("namespace").Value.String()
			printApplicationList(ctx, c, appName, namespace)
		},
	}

	cmd.PersistentFlags().StringP("app", "a", "", "Application name")
	return cmd
}

func printApplicationList(ctx context.Context, c client.Client, appName string, namespace string) {
	applicationMetaList, err := cmdutil.RetrieveApplicationsByName(ctx, c, appName, namespace)

	table := uitable.New()
	table.MaxColWidth = 60

	if err != nil {
		fmt.Printf("listing Trait Definition hit an issue: %s\n", err)
		return
	}

	table.AddRow("NAME", "WORKLOAD", "TRAITS", "STATUS", "CREATED-TIME")
	for _, a := range applicationMetaList {
		traitNames := strings.Join(a.Traits, ",")
		table.AddRow(a.Name, a.Workload, traitNames, a.Status, a.CreatedTime)
	}
	fmt.Print(table.String())
}
