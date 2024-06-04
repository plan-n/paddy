package grid

import (
	v1 "github.com/paddy/api/v1"
)

func GetPrimaryName(grid *v1.Grid) string {
	return grid.Spec.TargetRef.Name + "-primary"
}
