package cli

import (
	"os"
	"reflect"
	"testing"
	"unsafe"

	"github.com/spf13/cobra"
)

func TestMain(m *testing.M) {
	// Cobra's Execute() on a child command delegates to Root().ExecuteC(),
	// ignoring the child's SetArgs. This breaks tests that call Execute()
	// on subcommands found via the command tree. Detach the query command
	// so its Execute() runs directly, respecting SetArgs.
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "knowledge" {
			for _, sub := range cmd.Commands() {
				if sub.Name() == "query" {
					detachFromParent(sub)
				}
			}
		}
	}

	os.Exit(m.Run())
}

// detachFromParent clears a cobra.Command's unexported parent field.
// The command remains in its parent's Commands() list for discovery,
// but HasParent() returns false so Execute() runs directly.
func detachFromParent(cmd *cobra.Command) {
	v := reflect.ValueOf(cmd).Elem()
	f := v.FieldByName("parent")
	ptr := reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
	ptr.Set(reflect.Zero(f.Type()))
}
