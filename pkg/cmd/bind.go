package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	cmdutil "github.com/cloud-native-application/rudrx/pkg/cmd/util"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	corev1alpha2 "github.com/crossplane/oam-kubernetes-runtime/apis/core/v1alpha2"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type commandOptions struct {
	Env       *EnvMeta
	Template  cmdutil.Template
	Component corev1alpha2.Component
	AppConfig corev1alpha2.ApplicationConfiguration
	Client    client.Client
	cmdutil.IOStreams
}

func NewCommandOptions(ioStreams cmdutil.IOStreams) *commandOptions {
	return &commandOptions{IOStreams: ioStreams}
}

func NewBindCommand(f cmdutil.Factory, c client.Client, ioStreams cmdutil.IOStreams, args []string) *cobra.Command {

	var err error

	ctx := context.Background()

	o := NewCommandOptions(ioStreams)
	o.Env, err = GetEnv()
	if err != nil {
		fmt.Printf("Listing trait definitions hit an issue: %v\n", err)
		os.Exit(1)
	}
	o.Client = c
	cmd := &cobra.Command{
		Use:                   "bind APPLICATION-NAME TRAIT-NAME [FLAG]",
		DisableFlagsInUseLine: true,
		Short:                 "Attach a trait to a component",
		Long:                  "Attach a trait to a component.",
		Example:               `rudr bind frontend scaler --max=5`,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args, ctx))
			cmdutil.CheckErr(o.Run(f, cmd, ctx))
		},
	}
	cmd.SetArgs(args)
	var traitDefinitions corev1alpha2.TraitDefinitionList
	err = c.List(ctx, &traitDefinitions)
	if err != nil {
		fmt.Println("Listing trait definitions hit an issue:", err)
		os.Exit(1)
	}

	for _, t := range traitDefinitions.Items {
		var traitTemplate cmdutil.Template
		traitTemplate, err := cmdutil.ConvertTemplateJson2Object(t.Spec.Extension)
		if err != nil {
			fmt.Printf("extract template from traitDefinition %v err: %v, ignore it\n", t.Name, err)
			continue
		}

		for _, p := range traitTemplate.Parameters {
			if p.Type == "int" {
				v, err := strconv.Atoi(p.Default)
				if err != nil {
					fmt.Println("Parameters type is wrong: ", err, ".Please report this to OAM maintainer, thanks.")
					os.Exit(1)
				}
				cmd.PersistentFlags().Int(p.Name, v, p.Usage)
			} else {
				cmd.PersistentFlags().String(p.Name, p.Default, p.Usage)
			}
		}
	}

	return cmd
}

func (o *commandOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string, ctx context.Context) error {
	argsLength := len(args)
	var componentName string

	c := o.Client

	namespace := o.Env.Namespace

	if argsLength == 0 {
		return errors.New("please append the name of an application. Use `rudr bind -h` for more detailed information")
	} else if argsLength <= 2 {
		componentName = args[0]
		err := c.Get(ctx, client.ObjectKey{Namespace: o.Env.Namespace, Name: componentName}, &o.AppConfig)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		var component corev1alpha2.Component
		err = c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: componentName}, &component)
		if err != nil {
			errMsg := fmt.Sprintf("%s. Please choose an existed component name.", err)
			cmdutil.PrintErrorMessage(errMsg, 1)
		}

		// Retrieve all traits which can be used for the following 1) help and 2) validating
		traitList, _ := RetrieveTraitsByWorkload(ctx, o.Client, namespace, "")
		//if err != nil {
		//	errMsg := fmt.Sprintf("List available traits hit an issue: %s", err)
		//	cmdutil.PrintErrorMessage(errMsg, 1)
		//}

		switch argsLength {
		case 1:
			// Validate component and suggest trait
			fmt.Print("Error: No trait specified.\nPlease choose a trait: ")
			for _, t := range traitList {
				n := t.Short
				if n == "" {
					n = t.Name
				}
				fmt.Print(n, " ")
			}
			os.Exit(1)

		case 2:
			// validate trait
			traitName := args[1]
			traitLongName, _, _ := cmdutil.GetTraitNameAliasKind(ctx, c, namespace, traitName)

			traitDefinition, err := cmdutil.GetTraitDefinitionByName(ctx, c, namespace, traitLongName)
			if err != nil {
				errMsg := fmt.Sprintf("trait name [%s] is not valid, please try again", traitName)
				cmdutil.PrintErrorMessage(errMsg, 1)
			}

			traitTemplate, err := cmdutil.ConvertTemplateJson2Object(traitDefinition.Spec.Extension)

			if err != nil {
				return fmt.Errorf("attaching the trait hit an issue: %s", err)
			}

			pvd := fieldpath.Pave(traitTemplate.Object.Object)
			for _, v := range traitTemplate.Parameters {
				flagSet := cmd.Flag(v.Name)
				for _, path := range v.FieldPaths {
					fValue := flagSet.Value.String()
					if v.Type == "int" {
						portValue, _ := strconv.ParseFloat(fValue, 64)
						pvd.SetNumber(path, portValue)
						continue
					}
					pvd.SetString(path, fValue)
				}
			}

			// metadata.name needs to be in lower case.
			pvd.SetString("metadata.name", strings.ToLower(traitName))

			var t corev1alpha2.ComponentTrait
			t.Trait.Object = &unstructured.Unstructured{Object: pvd.UnstructuredContent()}
			o.Component.Name = componentName
			o.AppConfig.Spec.Components = []corev1alpha2.ApplicationConfigurationComponent{{
				ComponentName: componentName,
				Traits:        []corev1alpha2.ComponentTrait{t},
			}}
		}
	} else {
		cmdutil.PrintErrorMessage("Unknown command is specified, please check and try again.", 1)
	}
	return nil
}

func (o *commandOptions) Run(f cmdutil.Factory, cmd *cobra.Command, ctx context.Context) error {
	fmt.Println("Applying trait for component", o.Component.Name)
	c := o.Client
	err := c.Update(ctx, &o.AppConfig)
	if err != nil {
		msg := fmt.Sprintf("Applying trait hit an issue: %s", err)
		cmdutil.PrintErrorMessage(msg, 1)
	}

	msg := fmt.Sprintf("Succeeded!")
	fmt.Println(msg)
	return nil
}
