package checker

import (
	"context"
	"fmt"

	"github.com/cavos-io/go-file-arch/internal/archlint/models"
	"github.com/cavos-io/go-file-arch/internal/archlint/models/arch"
)

type CompositeChecker struct {
	checkers []checker
}

func NewCompositeChecker(checkers ...checker) *CompositeChecker {
	return &CompositeChecker{checkers: checkers}
}

func (c *CompositeChecker) Check(ctx context.Context, spec arch.Spec) (models.CheckResult, error) {
	overallResults := models.CheckResult{}

	for ind, checker := range c.checkers {
		results, err := checker.Check(ctx, spec)
		if err != nil {
			return models.CheckResult{}, fmt.Errorf("checker failed '%T': %w", checker, err)
		}

		overallResults.Append(results)

		if results.HasNotices() && ind < len(c.checkers)-1 {
			break
		}
	}

	return overallResults, nil
}
