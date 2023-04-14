package api

import policyv1 "github.com/Gentleelephant/custom-controller/pkg/apis/policy/v1"

func init() {
	AddToSchemes = append(AddToSchemes, policyv1.AddToScheme)
}
