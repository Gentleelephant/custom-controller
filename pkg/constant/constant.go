package constant

const (
	ResourceDistributionId = "distribution.kubesphere.io/id"

	ResourceDistribution = "distribution.kubesphere.io/policy"

	ResourceDistributionRuleId = "distribution.kubesphere.io/ruleid"

	DistributionManaged = "distribution.kubesphere.io/managed"

	SyncCluster = "distribution.kubesphere.io/cluster"

	WorkloadName = "distribution.kubesphere.io/workload"

	Finalizer = "distribution.kubesphere.io/finalizer"

	HostCluster = "cluster-role.kubesphere.io/host"

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
