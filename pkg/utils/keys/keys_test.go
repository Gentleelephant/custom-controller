package keys

import (
	"testing"
)

func TestName(t *testing.T) {

	rdKey := ClusterWideKey{
		Group:     "apps",
		Version:   "v1",
		Kind:      "Deployment",
		Namespace: "default",
		Name:      "",
	}

	tests := []ClusterWideKey{
		{
			Group:     "apps",
			Version:   "v1",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "xxxx",
		},
		{
			Group:     "apps",
			Version:   "v1",
			Kind:      "Service",
			Namespace: "default",
			Name:      "sss",
		},
		{
			Group:     "apps",
			Version:   "v1",
			Kind:      "",
			Namespace: "",
			Name:      "",
		},
	}

	for _, test := range tests {
		match := PrefixMatch(test, rdKey)
		t.Logf("match: %v", match)
	}

}
