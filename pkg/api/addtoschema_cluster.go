package api

import "github.com/Gentleelephant/custom-controller/pkg/apis/cluster/v1alpha1"

func init() {
	AddToSchemes = append(AddToSchemes, v1alpha1.AddToScheme)
}
