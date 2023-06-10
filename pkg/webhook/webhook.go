package webhook

import (
	"context"
	"encoding/json"
	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/distribution/v1"
	"k8s.io/klog/v2"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/mutate-v1-rd,mutating=true,failurePolicy=fail,sideEffects=None,groups=distribution.kubesphere.io,resources=resourcedistributions,verbs=create;update,versions=v1,name=mdistribution.kb.io,admissionReviewVersions=v1

type ResourceDistributionWebhook struct {
	Client  client.Client
	decoder *admission.Decoder
}

func (a *ResourceDistributionWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {

	klog.Info("Handle webhook request")
	rd := &v1.ResourceDistribution{}
	err := a.decoder.Decode(req, rd)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// mutate the fields in pod

	marshaledRd, err := json.Marshal(rd)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	klog.Info("Handle webhook request success")
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledRd)

}

func (a *ResourceDistributionWebhook) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}