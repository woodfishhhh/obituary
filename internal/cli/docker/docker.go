package docker

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "docker", Short: "Give Docker containers a lifespan", Long: "Track Docker containers by immutable full ID and creation time. Expiry stops the container and retains its writable layer. Resurrection starts it. After [graveyard] retention_days, purge removes stopped containers without removing volumes. This also applies in recycle mode. Images, volumes and networks are preserved."}
	cmd.AddCommand(newAdd(), newList())
	return cmd
}

func newAdd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{Use: "add <container name|ID> <lifespan>", Short: "Register one Docker container; expiry stops it", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		d, err := core.ParseDuration(args[1])
		if err != nil {
			return err
		}
		svc, err := cmdutil.Open()
		if err != nil {
			return err
		}
		item, err := svc.WithContext(cmd.Context()).AddContainer(args[0], core.AddOptions{ExpiresAt: time.Now().Add(d)})
		if err := cmdutil.Committed(err); err != nil {
			return err
		}
		cmdutil.EnsureDaemon(svc.Store.Home, asJSON)
		if asJSON {
			return cmdutil.PrintJSON(item)
		}
		fmt.Printf("#%d %s  expires %s\n", item.ID, item.Path, cmdutil.When(item.ExpiresAt))
		return nil
	}}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

func newList() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{Use: "list", Short: "List registered Docker containers", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := cmdutil.Open()
		if err != nil {
			return err
		}
		st, err := svc.Store.Read()
		if err != nil {
			return err
		}
		items := []core.Item{}
		for _, it := range st.Items {
			if it.Kind == core.KindContainer {
				items = append(items, it)
			}
		}
		if asJSON {
			return cmdutil.PrintJSON(items)
		}
		if len(items) == 0 {
			fmt.Println("no containers are tracked")
		}
		for _, it := range items {
			fmt.Printf("#%d %s  expires %s\n", it.ID, it.Path, cmdutil.When(it.ExpiresAt))
		}
		return nil
	}}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
