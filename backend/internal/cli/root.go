// Package cli builds the application's development command tree.
package cli

import (
	"github.com/spf13/cobra"

	modulecmd "github.com/abysstract/abysstract-admin/backend/internal/cli/module"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "admin", Short: "Application development tools", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(modulecmd.NewCommand())
	return cmd
}
