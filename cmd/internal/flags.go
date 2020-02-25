package internal

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// StringFlag initializes a string flag
func StringFlag(cmd *cobra.Command, name, description, value string) {
	cmd.Flags().String(name, value, description)
	viper.BindPFlag(name, cmd.Flags().Lookup(name)) // nolint: errcheck, gas
}

// BoolFlag initializes a bool flag
func BoolFlag(cmd *cobra.Command, name, description string, value bool) {
	cmd.Flags().Bool(name, value, description)
	viper.BindPFlag(name, cmd.Flags().Lookup(name)) // nolint: errcheck, gas
}

// Int64Flag initializes a int64 flag
func Int64Flag(cmd *cobra.Command, name, description string, value int64) {
	cmd.Flags().Int64(name, value, description)
	viper.BindPFlag(name, cmd.Flags().Lookup(name)) // nolint: errcheck, gas
}

// FlagChecker defines the function used to validate flags
type FlagChecker func() error

// CheckFlags validates a slice of flag checkers
func CheckFlags(checkers ...FlagChecker) error {
	var fails []string
	for _, checker := range checkers {
		if err := checker(); err != nil {
			fails = append(fails, err.Error())
		}
	}
	if len(fails) > 0 {
		return errors.New(strings.Join(fails, "\n"))
	}

	return nil
}

// RequireString returns an error if the given setting is not a string
func RequireString(flag string) FlagChecker {
	return func() error {
		v := viper.GetString(flag)
		if v == "" {
			return fmt.Errorf("flag %s can not be an empty string", flag)
		}
		return nil
	}
}
