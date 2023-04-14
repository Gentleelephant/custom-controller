package api

import workv1 "github.com/Gentleelephant/custom-controller/pkg/apis/work/v1"

func init() {
	AddToSchemes = append(AddToSchemes, workv1.AddToScheme)
}
