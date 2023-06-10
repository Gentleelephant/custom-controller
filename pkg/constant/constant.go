package constant

const (
	ResourceDistributionId = "distribution.kubesphere.io/id"

	ResourceDistributionPolicy = "distribution.kubesphere.io/policy"

	SyncCluster = "distribution.kubesphere.io/cluster"

	Finalizer = "distribution.kubesphere.io/finalizer"

	ResourceDistributionAnnotation = "distribution.kubesphere.io/rd"

	WorkloadCLusterAnnotation = "distribution.kubesphere.io/workload-cluster"

	// SuccessSynced is used as part of the Event 'reason' when a Foo is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"
	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	// MessageResourceSynced is the message used for an Event fired when a Foo
	// is synced successfully
	MessageResourceSynced = "ResourceDistribution synced successfully"
)
