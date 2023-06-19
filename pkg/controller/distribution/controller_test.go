package distribution

import (
	"fmt"
	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/distribution/v1alpha1"
	"github.com/duke-git/lancet/v2/random"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestChanged(t *testing.T) {

	old := &v1.ResourceDistribution{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ResourceDistribution",
			APIVersion: "distribution.k8s.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "old",
			Namespace: "old",
		},
		Spec: v1.ResourceDistributionSpec{
			ResourceSelector: v1.ResourceSelector{
				APIVersion: "apps/v1alpha1",
				Kind:       "Deployment",
				Namespace:  "default",
				Name:       "nginx-deployment",
			},
			Placement: &v1.Placement{
				ClusterAffinity: &v1.ClusterAffinity{
					LabelSelector: nil,
					ClusterNames:  []string{"cluster1", "cluster2"},
				},
			},
			OverrideRules: []v1.RuleWithCluster{
				{
					Id:            "rule1",
					TargetCluster: nil,
					Overriders: v1.Overriders{
						Plaintext: []v1.PlaintextOverrider{
							{
								Operator: "replace",
								Path:     "/spec/template/spec/containers/0/image",
								Value: apiextensionsv1.JSON{
									Raw: []byte(`"nginx:1.16.1"`),
								},
							},
						},
					},
				},
			},
		},
	}

	new := &v1.ResourceDistribution{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ResourceDistribution",
			APIVersion: "distribution.k8s.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "new",
			Namespace: "new",
		},
		Spec: v1.ResourceDistributionSpec{
			ResourceSelector: v1.ResourceSelector{
				APIVersion: "apps/v1alpha1",
				Kind:       "Deployment",
				Namespace:  "default",
				Name:       "nginx-deployment",
			},
			Placement: &v1.Placement{
				ClusterAffinity: &v1.ClusterAffinity{
					LabelSelector: nil,
					ClusterNames:  []string{"cluster1", "cluster2", "cluster3"},
				},
			},
			OverrideRules: []v1.RuleWithCluster{
				{
					Id:            "rule1",
					TargetCluster: nil,
					Overriders: v1.Overriders{
						Plaintext: []v1.PlaintextOverrider{
							{
								Operator: "replace",
								Path:     "/spec/template/spec/containers/0/image",
								Value: apiextensionsv1.JSON{
									Raw: []byte(`"nginx:1.16.1"`),
								},
							},
							{
								Operator: "replace",
								Path:     "/spec/template/spec/containers/0/image",
								Value: apiextensionsv1.JSON{
									Raw: []byte(`"nginx:1.16.2"`),
								},
							},
						},
					},
				},
			},
		},
	}

	b, b2, b3 := changes(old, new)
	fmt.Println(b, b2, b3)

}
