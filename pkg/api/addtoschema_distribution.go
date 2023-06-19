package api

import syncv1 "github.com/Gentleelephant/custom-controller/pkg/apis/distribution/v1alpha1"

func init() {
	AddToSchemes = append(AddToSchemes, syncv1.AddToScheme)
}
