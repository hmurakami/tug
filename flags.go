package main

// stringSlice implements flag.Value for repeatable string flags.
type stringSlice []string

func (s *stringSlice) String() string { return "" }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type globalFlags struct {
	composeFile   string
	overrideFiles stringSlice
}
