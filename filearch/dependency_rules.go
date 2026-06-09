package filearch

import "fmt"

type DependencyRule struct {
	MayDependOn []string `yaml:"mayDependOn"`
}

func (cfg *Config) validateDependencyRules() error {
	for component := range cfg.DependencyRules {
		if _, ok := cfg.Components[component]; !ok {
			return fmt.Errorf("dependency rule references undefined component %q", component)
		}
		for _, dependency := range cfg.DependencyRules[component].MayDependOn {
			if _, ok := cfg.Components[dependency]; !ok {
				return fmt.Errorf("dependency rule for %q references undefined component %q", component, dependency)
			}
		}
	}
	return nil
}

// TODO: enforce dependencyRules by mapping each file's imports to configured
// components and reporting imports whose target component is not allowed.
func checkDependencyRules(*Config) error {
	return nil
}
