package validator

import (
	"strings"

	"github.com/Thitipong-PP/palet/internal/config"
)

// MissingArgs returns the names of required args that have no value.
func MissingArgs(cmd config.Command, vals map[string]string) []string {
	missing := make([]string, 0, len(cmd.Args))
	for _, a := range cmd.Args {
		if a.Required && strings.TrimSpace(vals[a.Name]) == "" {
			missing = append(missing, a.Name)
		}
	}
	return missing
}
