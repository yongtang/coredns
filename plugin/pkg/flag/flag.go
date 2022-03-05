// Package flag is used to return a FlagSet that can be used across
// the application.
package flag

import (
	"flag"
	"os"
	"sync"
)

var (
	o sync.Once
	f *flag.FlagSet
)

// FlagSet returns a FlagSet that is shared across the application.
func FlagSet() *flag.FlagSet {
	o.Do(func() {
		f = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	})
	return f
}
