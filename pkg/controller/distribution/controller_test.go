package distribution

import (
	"github.com/duke-git/lancet/v2/random"
	"testing"
)

func TestName(t *testing.T) {

	lower1 := random.RandLower(8)
	lower2 := random.RandLower(8)
	lower3 := random.RandLower(8)

	t.Log(lower1)
	t.Log(lower2)
	t.Log(lower3)

}
