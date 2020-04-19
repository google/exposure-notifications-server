package flag

import (
	"fmt"
	"strings"
)

// RegionListVar is a list of upper-cased, unique regions derived from a comma-separated list.
type RegionListVar []string

func (l *RegionListVar) String() string {
	return fmt.Sprint(*l)
}

// Set parses the flag value into the final result.
func (l *RegionListVar) Set(val string) error {
	if len(*l) > 0 {
		return fmt.Errorf("already set")
	}

	unique := map[string]struct{}{}
	for _, v := range strings.Split(val, ",") {
		vf := strings.ToUpper(strings.TrimSpace(v))
		if _, seen := unique[vf]; !seen {
			*l = append(*l, vf)
			unique[vf] = struct{}{}
		}
	}
	return nil
}
