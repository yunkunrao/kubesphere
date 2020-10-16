/*
Copyright 2019 The KubeSphere Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package prometheus

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/klog"
	"kubesphere.io/kubesphere/pkg/simple/client/monitoring"
)

var promQLMeterTemplates = map[string]string{
	// cluster
	"meter_cluster_cpu_usage": `
round(
	(
		sum(
			sum_over_time(avg_over_time(kube_pod_container_resource_requests{resource="cpu",unit="core"}[1h])[$step:1h])
		) >=
		(
			sum_over_time(avg_over_time(:node_cpu_utilisation:avg1m[1h])[$step:1h]) * 
			sum(
				sum_over_time(avg_over_time(node:node_num_cpu:sum[1h])[$step:1h])
			)
		)
	)
	or
	(
		(
			sum_over_time(avg_over_time(:node_cpu_utilisation:avg1m[1h])[$step:1h]) * 
			sum(
				sum_over_time(avg_over_time(node:node_num_cpu:sum[1h])[$step:1h])
			)
		) >
		sum(
			sum_over_time(avg_over_time(kube_pod_container_resource_requests{resource="cpu",unit="core"}[1h])[$step:1h])
		)
	),
	0.001
)`,

	"meter_cluster_memory_usage": `
round(
	(
		sum(
			sum_over_time(avg_over_time(kube_pod_container_resource_requests{resource="memory",unit="byte"}[1h])[$step:1h])
		) >=
		(
			sum_over_time(avg_over_time(:node_memory_utilisation:[1h])[$step:1h]) * 
			sum(
				sum_over_time(avg_over_time(node:node_memory_bytes_total:sum[1h])[$step:1h])
			)
		)
	)
	or
	(
		(
			sum_over_time(avg_over_time(:node_memory_utilisation:[1h])[$step:1h]) * 
			sum(
				sum_over_time(avg_over_time(node:node_memory_bytes_total:sum[1h])[$step:1h])
			)
		) >
		sum(
			sum_over_time(avg_over_time(kube_pod_container_resource_requests{resource="memory",unit="byte"}[1h])[$step:1h])
		)
	),
	1
)`,

	"meter_cluster_net_bytes_transmitted": `
round(
	sum(
		sum_over_time(
			increase(
				node_network_transmit_bytes_total{
					job="node-exporter",
					device!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)"
				}[1h]
			)[$step:1h]
		)
	),
	1
)`,

	"meter_cluster_net_bytes_received": `
round(
	sum(
		sum_over_time(
			increase(
				node_network_receive_bytes_total{
					job="node-exporter",
					device!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)"
				}[1h]
			)[$step:1h]
		)
	),
	1
)`,

	"meter_cluster_pvc_bytes_total": `
sum(
	topk(1, sum_over_time(avg_over_time(namespace:pvc_bytes_pod:sum{}[1h])[$step:1h])) by (persistentvolumeclaim)
)`,

	// node
	"meter_node_cpu_usage": `
round(
	(
		sum(
			sum_over_time(avg_over_time(kube_pod_container_resource_requests{$nodeSelector, resource="cpu",unit="core"}[1h])[$step:1h])
		) by (node) >=
		sum(
			sum_over_time(avg_over_time(node:node_cpu_utilisation:avg1m{$nodeSelector}[1h])[$step:1h]) * 
			sum_over_time(avg_over_time(node:node_num_cpu:sum{$nodeSelector}[1h])[$step:1h])
		) by (node)
	)
	or
	(
		sum(
			sum_over_time(avg_over_time(node:node_cpu_utilisation:avg1m{$nodeSelector}[1h])[$step:1h]) * 
			sum_over_time(avg_over_time(node:node_num_cpu:sum{$nodeSelector}[1h])[$step:1h])
		) by (node) >
		sum(
			sum_over_time(avg_over_time(kube_pod_container_resource_requests{$nodeSelector, resource="cpu",unit="core"}[1h])[$step:1h])
		) by (node)
	)
	or
	(
		sum(
			sum_over_time(avg_over_time(node:node_cpu_utilisation:avg1m{$nodeSelector}[1h])[$step:1h]) * 
			sum_over_time(avg_over_time(node:node_num_cpu:sum{$nodeSelector}[1h])[$step:1h])
		) by (node)
	),
	0.001
)`,

	"meter_node_memory_usage_wo_cache": `
round(
	(
		sum(
			sum_over_time(avg_over_time(kube_pod_container_resource_requests{$nodeSelector, resource="memory",unit="byte"}[1h])[$step:1h])
		) by (node) >=
		sum(
			sum_over_time(avg_over_time(node:node_memory_bytes_total:sum{$nodeSelector}[1h])[$step:1h]) -
			sum_over_time(avg_over_time(node:node_memory_bytes_available:sum{$nodeSelector}[1h])[$step:1h])
		) by (node)
	)
	or
	(
		sum(
			sum_over_time(avg_over_time(node:node_memory_bytes_total:sum{$nodeSelector}[1h])[$step:1h]) -
			sum_over_time(avg_over_time(node:node_memory_bytes_available:sum{$nodeSelector}[1h])[$step:1h])
		) by (node) >
		sum(
			sum_over_time(avg_over_time(kube_pod_container_resource_requests{$nodeSelector, resource="memory",unit="byte"}[1h])[$step:1h])
		) by (node)
	)
	or
	(
		sum(
			sum_over_time(avg_over_time(node:node_memory_bytes_total:sum{$nodeSelector}[1h])[$step:1h]) -
			sum_over_time(avg_over_time(node:node_memory_bytes_available:sum{$nodeSelector}[1h])[$step:1h])
		) by (node)
	),
	0.001
)`,

	"meter_node_net_bytes_transmitted": `
round(
	sum by (node) (
		sum without (instance) (
			label_replace(
				sum_over_time(
					increase(
						node_network_transmit_bytes_total{
							job="node-exporter",
							device!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)",
							$instanceSelector
						}[1h]
					)[$step:1h]
				),
				"node",
				"$1",
				"instance",
				"(.*)"
			)
		)
	),
	1
)`,

	"meter_node_net_bytes_received": `
round(
	sum by (node) (
		sum without (instance) (
			label_replace(
				sum_over_time(
					increase(
						node_network_receive_bytes_total{
							job="node-exporter",
							device!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)",
							$instanceSelector
						}[1h]
					)[$step:1h]
				),
				"node", "
				$1",
				"instance",
				"(.*)"
			)
		)
	),
	1
)`,

	"meter_node_pvc_bytes_total": `
sum(
	topk(
		1,
		sum_over_time(
			avg_over_time(
				namespace:pvc_bytes_pod:sum{$nodeSelector}[1h]
			)[$step:1h]
		)
	) by (persistentvolumeclaim, node)
) by (node)`,

	// workspace
	"meter_workspace_cpu_usage": `
round(
	(
		sum by (workspace) (
			sum_over_time(
				avg_over_time(
					namespace:kube_pod_resource_request:sum{
						owner_kind!="Job",
						namespace!="",
						resource="cpu",
						$1
					}[1h]
				)[$step:1h]
			)
		) >=
		sum by (workspace) (
			sum_over_time(
				avg_over_time(namespace:container_cpu_usage_seconds_total:sum_rate{namespace!="", $1}[1h])[$step:1h]
			)
		)
	)
	or
	(
		sum by (workspace) (
			sum_over_time(
				avg_over_time(namespace:container_cpu_usage_seconds_total:sum_rate{namespace!="", $1}[1h])[$step:1h]
			)
		) >
		sum by (workspace) (
			sum_over_time(
				avg_over_time(
					namespace:kube_pod_resource_request:sum{
						owner_kind!="Job",
						namespace!="",
						resource="cpu",
						$1
					}[1h]
				)[$step:1h]
			)
		)
	)
	or
	(
		sum by (workspace) (
			sum_over_time(
				avg_over_time(namespace:container_cpu_usage_seconds_total:sum_rate{namespace!="", $1}[1h])[$step:1h]
			)
		)
	),
	0.001
)`,

	"meter_workspace_memory_usage": `
round(
	(
		sum by (workspace) (
			sum_over_time(
				avg_over_time(
					namespace:kube_pod_resource_request:sum{
						owner_kind!="Job",
						namespace!="",
						resource="memory",
						$1
					}[1h]
				)[$step:1h]
			)
		) >=
		sum by (workspace) (
			sum_over_time(avg_over_time(namespace:container_memory_usage_bytes:sum{namespace!="", $1}[1h])[$step:1h])
		)
	)
	or
	(
		sum by (workspace) (
			sum_over_time(avg_over_time(namespace:container_memory_usage_bytes:sum{namespace!="", $1}[1h])[$step:1h])
		) >
		sum by (workspace) (
			sum_over_time(
				avg_over_time(
					namespace:kube_pod_resource_request:sum{
					owner_kind!="Job", namespace!="", resource="memory", $1
					}[1h]
				)[$step:1h]
			)
		)
	)
	or
	(
		sum by (workspace) (
			sum_over_time(avg_over_time(namespace:container_memory_usage_bytes:sum{namespace!="", $1}[1h])[$step:1h])
		)
	),
	1
)`,

	"meter_workspace_net_bytes_transmitted": `
round(
	sum by (workspace) (
		sum by (namespace) (
			sum_over_time(
				increase(
					container_network_transmit_bytes_total{
						namespace!="",
						pod!="",
						interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)",
						job="kubelet"
					}[1h]
				)[$step:1h]
			)
		) * on (namespace) group_left(workspace)
		kube_namespace_labels{$1}
	) or on(workspace) max by(workspace) (kube_namespace_labels{$1} * 0), 1)`,

	"meter_workspace_net_bytes_received": `
round(
	sum by (workspace) (
		sum by (namespace) (
			sum_over_time(
				increase(
					container_network_receive_bytes_total{
						namespace!="",
						pod!="",
						interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)",
						job="kubelet"
					}[1h]
				)[$step:1h]
			)
		) * on (namespace) group_left(workspace)
		kube_namespace_labels{$1}
	) or on(workspace) max by(workspace) (kube_namespace_labels{$1} * 0), 1)`,

	"meter_workspace_pvc_bytes_total": `
sum (
	topk(
		1,
		sum_over_time(
			avg_over_time(namespace:pvc_bytes_pod:sum{$1}[1h])[$step:1h]
		)
	) by (persistentvolumeclaim, workspace)
) by (workspace)`,

	// namespace
	"meter_namespace_cpu_usage": `
round(
	(
		sum by (namespace) (
			sum_over_time(
				avg_over_time(
					namespace:kube_pod_resource_request:sum{
						owner_kind!="Job",
						namespace!="",
						resource="cpu",
						$1
					}[1h]
				)[$step:1h]
			)
		) >=
		sum by (namespace) (
			sum_over_time(
				avg_over_time(namespace:container_cpu_usage_seconds_total:sum_rate{namespace!="", $1}[1h])[$step:1h]
			)
		)
	)
	or
	(
		sum by (namespace) (
			sum_over_time(avg_over_time(namespace:container_cpu_usage_seconds_total:sum_rate{namespace!="", $1}[1h])[$step:1h])
		) >
		sum by (namespace) (
			sum_over_time(
				avg_over_time(
					namespace:kube_pod_resource_request:sum{owner_kind!="Job", namespace!="", resource="cpu", $1}[1h]
				)[$step:1h]
			)
		)
	)
	or
	(
		sum by (namespace) (
			sum_over_time(
				avg_over_time(
					namespace:container_cpu_usage_seconds_total:sum_rate{namespace!="", $1}[1h]
				)[$step:1h]
			)
		)
	),
	0.001
)`,

	"meter_namespace_memory_usage_wo_cache": `
round(
	(
		sum by (namespace) (
			sum_over_time(
				avg_over_time(
					namespace:kube_pod_resource_request:sum{
						owner_kind!="Job",
						namespace!="",
						resource="memory",
						$1
					}[1h]
				)[$step:1h]
			)
		) >=
		sum by (namespace) (
			sum_over_time(avg_over_time(namespace:container_memory_usage_bytes_wo_cache:sum{namespace!="", $1}[1h])[$step:1h])
		)
	)
	or
	(
		sum by (namespace) (
			sum_over_time(avg_over_time(namespace:container_memory_usage_bytes_wo_cache:sum{namespace!="", $1}[1h])[$step:1h])
		) >
		sum by (namespace) (
			sum_over_time(
				avg_over_time(
					namespace:kube_pod_resource_request:sum{
						owner_kind!="Job", namespace!="", resource="memory", $1
					}[1h]
				)[$step:1h]
			)
		)
	)
	or
	(
		sum by (namespace) (
			sum_over_time(
				avg_over_time(
					namespace:container_memory_usage_bytes_wo_cache:sum{namespace!="", $1}[1h]
				)[$step:1h]
			)
		)
	),
	1
)`,

	"meter_namespace_net_bytes_transmitted": `
round(
	sum by (namespace) (
		sum_over_time(
			increase(
				container_network_transmit_bytes_total{
					namespace!="",
					pod!="",
					interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)",
					job="kubelet"
				}[1h]
			)[$step:1h]
		)
		* on (namespace) group_left(workspace)
		kube_namespace_labels{$1}
	)
	or on(namespace) max by(namespace) (kube_namespace_labels{$1} * 0), 1)`,

	"meter_namespace_net_bytes_received": `
round(
	sum by (namespace) (
		sum_over_time(
			increase(
				container_network_receive_bytes_total{
					namespace!="",
					pod!="",
					interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)",
					job="kubelet"
				}[1h]
			)[$step:1h]
		)
		* on (namespace) group_left(workspace)
		kube_namespace_labels{$1}
	)
	or on(namespace) max by(namespace) (kube_namespace_labels{$1} * 0), 1)`,

	"meter_namespace_pvc_bytes_total": `
sum (
	topk(
		1,
		sum_over_time(
			avg_over_time(namespace:pvc_bytes_pod:sum{$1}[1h])[$step:1h]
		)
	) by (persistentvolumeclaim, namespace)
) by (namespace)`,

	// application
	"meter_application_cpu_usage": `
round(
	(
		sum by (namespace, application) (
			label_replace(
				sum_over_time(
					avg_over_time(
						namespace:kube_workload_resource_request:sum{workload!~"Job:.+", resource="cpu", $1}[1h]
					)[$step:1h]
				),
				"application",
				"$app",
				"",
				""
			)
		) >=
		sum by (namespace, application) (
			label_replace(
				sum_over_time(avg_over_time(namespace:workload_cpu_usage:sum{$1}[1h])[$step:1h]),
				"application",
				"$app",
				"",
				""
			)
		)
	)
	or
	(
		sum by (namespace, application) (
			label_replace(
				sum_over_time(avg_over_time(namespace:workload_cpu_usage:sum{$1}[1h])[$step:1h]),
				"application",
				"$app",
				"",
				""
			)
		) >
		sum by (namespace, application) (
			label_replace(
				sum_over_time(
					avg_over_time(
						namespace:kube_workload_resource_request:sum{workload!~"Job:.+", resource="cpu", $1}[1h]
					)[$step:1h]
				),
				"application",
				"$app",
				"",
				""
			)
		)
	)
	or
	(
		sum by (namespace, application) (
			label_replace(
				sum_over_time(avg_over_time(namespace:workload_cpu_usage:sum{$1}[1h])[$step:1h]),
				"application",
				"$app",
				"",
				""
			)
		)
	),
	0.001
)`,

	"meter_application_memory_usage_wo_cache": `
round(
	(
		sum by (namespace, application) (
			label_replace(
				sum_over_time(
					avg_over_time(
						namespace:kube_workload_resource_request:sum{workload!~"Job:.+", resource="memory", $1}[1h]
					)[$step:1h]
				),
				"application",
				"$app",
				"",
				""
			)
		) >=
		sum by (namespace, application) (
			label_replace(
				sum_over_time(avg_over_time(namespace:workload_memory_usage_wo_cache:sum{$1}[1h])[$step:1h]),
				"application",
				"$app",
				"",
				""
			)
		)
	)
	or
	(
		sum by (namespace, application) (
			label_replace(
				sum_over_time(avg_over_time(namespace:workload_memory_usage_wo_cache:sum{$1}[1h])[$step:1h]),
				"application",
				"$app",
				"",
				""
			)
		) >
		sum by (namespace, application) (
			label_replace(
				sum_over_time(
					avg_over_time(
						namespace:kube_workload_resource_request:sum{workload!~"Job:.+", resource="memory", $1}[1h]
					)[$step:1h]
				),
				"application",
				"$app",
				"",
				""
			)
		)
	)
	or
	(
		sum by (namespace, application) (
			label_replace(
				sum_over_time(avg_over_time(namespace:workload_memory_usage_wo_cache:sum{$1}[1h])[$step:1h]),
				"application",
				"$app",
				"",
				""
			)
		)
	),
	1
)`,

	"meter_application_net_bytes_transmitted": `
round(
	sum by (namespace, application) (
		label_replace(
			sum_over_time(
				increase(
					namespace:workload_net_bytes_transmitted:sum{$1}[1h]
				)[$step:1h]
			),
			"application",
			"$app",
			"",
			""
		)
	),
	1
)`,

	"meter_application_net_bytes_received": `
sum by (namespace, application) (
	label_replace(
		sum_over_time(
			increase(
				namespace:workload_net_bytes_received:sum{$1}[1h]
			)[$step:1h]
		),
		"application",
		"$app",
		"",
		""
	)
)`,

	"meter_application_pvc_bytes_total": `
sum by (namespace, application) (
	label_replace(
		topk(1, sum_over_time(avg_over_time(namespace:pvc_bytes_pod:sum{$1}[1h])[$step:1h])) by (persistentvolumeclaim),
		"application",
		"$app",
		"",
		""
	)
)`,

	// workload
	"meter_workload_cpu_usage": `
round(
	(
		sum by (namespace, workload) (
			sum_over_time(
				avg_over_time(
					namespace:kube_workload_resource_request:sum{
						workload!~"Job:.+", resource="cpu", $1
					}[1h]
				)[$step:1h]
			)
		) >=
		sum by (namespace, workload) (
			sum_over_time(avg_over_time(namespace:workload_cpu_usage:sum{$1}[1h])[$step:1h])
		)
	)
	or
	(
		sum by (namespace, workload) (
			sum_over_time(avg_over_time(namespace:workload_cpu_usage:sum{$1}[1h])[$step:1h])
		) >
		sum by (namespace, workload) (
			sum_over_time(
				avg_over_time(
					namespace:kube_workload_resource_request:sum{
						workload!~"Job:.+", resource="cpu", $1
					}[1h]
				)[$step:1h]
			)
		)
	)
	or
	(
		sum by (namespace, workload) (
			sum_over_time(avg_over_time(namespace:workload_cpu_usage:sum{$1}[1h])[$step:1h])
		)
	),
	0.001
)`,

	"meter_workload_memory_usage_wo_cache": `
round(
	(
		sum by (namespace, workload) (
			sum_over_time(
				avg_over_time(
					namespace:kube_workload_resource_request:sum{
						workload!~"Job:.+", resource="memory", $1
					}[1h]
				)[$step:1h]
			)
		) >=
		sum by (namespace, workload) (
			sum_over_time(avg_over_time(namespace:workload_memory_usage_wo_cache:sum{$1}[1h])[$step:1h])
		)
	)
	or
	(
		sum by (namespace, workload) (
			sum_over_time(avg_over_time(namespace:workload_memory_usage_wo_cache:sum{$1}[1h])[$step:1h])
		) >
		sum by (namespace, workload) (
			sum_over_time(
				avg_over_time(
					namespace:kube_workload_resource_request:sum{
						workload!~"Job:.+", resource="memory", $1
					}[1h]
				)[$step:1h]
			)
		)
	)
	or
	(
		sum by (namespace, workload) (
			sum_over_time(avg_over_time(namespace:workload_memory_usage_wo_cache:sum{$1}[1h])[$step:1h])
		)
	),
	1
)`,

	"meter_workload_net_bytes_transmitted": `
round(
	sum_over_time(
		increase(
			namespace:workload_net_bytes_transmitted:sum{$1}[1h]
		)[$step:1h]
	),
	1
)`,

	"meter_workload_net_bytes_received": `
round(
	sum_over_time(
		increase(
			namespace:workload_net_bytes_received:sum{$1}[1h]
		)[$step:1h]
	),
	1
)`,

	"meter_workload_pvc_bytes_total": `
sum by (namespace, workload) (
	topk(
		1,
		sum_over_time(avg_over_time(namespace:pvc_bytes_pod:sum{$1}[1h])[$step:1h])
	) by (persistentvolumeclaim, namespace, workload)
)`,

	// service
	"meter_service_cpu_usage": `
round(
	sum by (namespace, service) (
		label_replace(
			sum by (namespace, pod) (
				sum_over_time(
					avg_over_time(
						namespace:kube_pod_resource_request:sum{owner_kind!="Job", resource="cpu", $1}[1h]
					)[$step:1h]
				)
			) >=
			sum by (namespace, pod) (
				sum by (namespace, pod) (
					sum_over_time(
						irate(
							container_cpu_usage_seconds_total{job="kubelet", pod!="", image!=""}[1h]
						)[$step:1h]
					)
				) * on (namespace, pod) group_left(owner_kind, owner_name)
				kube_pod_owner{} * on (namespace, pod) group_left(node)
				kube_pod_info{$1}
			),
			"service",
			"$svc",
			"",
			""
		)
	)
	or
	sum by (namespace, service) (
		label_replace(
			sum by (namespace, pod) (
				sum by (namespace, pod) (
					sum_over_time(
						irate(
							container_cpu_usage_seconds_total{job="kubelet", pod!="", image!=""}[1h]
						)[$step:1h]
					)
				) * on (namespace, pod) group_left(owner_kind, owner_name)
				kube_pod_owner{} * on (namespace, pod) group_left(node)
				kube_pod_info{$1}
			) >
			sum by (namespace, pod) (
				sum_over_time(
					avg_over_time(namespace:kube_pod_resource_request:sum{owner_kind!="Job", resource="cpu", $1}[1h])[$step:1h]
				)
			),
			"service",
			"$svc",
			"",
			""
		)
	)
	or
	sum by (namespace, service) (
		label_replace(
			sum by (namespace, pod) (
				sum by (namespace, pod) (
					sum_over_time(
						irate(
							container_cpu_usage_seconds_total{job="kubelet", pod!="", image!=""}[1h]
						)[$step:1h]
					)
				) * on (namespace, pod) group_left(owner_kind, owner_name)
				kube_pod_owner{} * on (namespace, pod) group_left(node)
				kube_pod_info{$1}
			),
			"service",
			"$svc",
			"",
			""
		)
	),
	0.001
)`,

	"meter_service_memory_usage_wo_cache": `
round(
	(
		sum by (namespace, service) (
			label_replace(
				sum by (namespace, pod) (
					sum_over_time(
						avg_over_time(
							namespace:kube_pod_resource_request:sum{owner_kind!="Job", resource="memory", $1}[1h]
						)[$step:1h]
					)
				) >=
				sum by (namespace, pod) (
					sum by (namespace, pod) (
						sum_over_time(
							avg_over_time(
								container_memory_working_set_bytes{job="kubelet", pod!="", image!=""}[1h]
							)[$step:1h]
						)
					) * on (namespace, pod) group_left(owner_kind, owner_name)
					kube_pod_owner{} * on (namespace, pod) group_left(node)
					kube_pod_info{$1}
				),
				"service",
				"$svc",
				"",
				""
			)
		)
	)
	or
	(
		sum by (namespace, service) (
			label_replace(
				sum by (namespace, pod) (
					sum by (namespace, pod) (
						sum_over_time(
							avg_over_time(
								container_memory_working_set_bytes{job="kubelet", pod!="", image!=""}[1h]
							)[$step:1h]
						)
					) * on (namespace, pod) group_left(owner_kind, owner_name)
					kube_pod_owner{} * on (namespace, pod) group_left(node)
					kube_pod_info{$1}
				) >
				sum by (namespace, pod) (
					sum_over_time(
						avg_over_time(
							namespace:kube_pod_resource_request:sum{owner_kind!="Job", resource="memory", $1}[1h]
						)[$step:1h]
					)
				),
				"service",
				"$svc",
				"",
				""
			)
		)
	)
	or
	(
		sum by (namespace, service) (
			label_replace(
				sum by (namespace, pod) (
					sum by (namespace, pod) (
						sum_over_time(
							avg_over_time(
								container_memory_working_set_bytes{job="kubelet", pod!="", image!=""}[1h]
							)[$step:1h]
						)
					) * on (namespace, pod) group_left(owner_kind, owner_name)
					kube_pod_owner{} * on (namespace, pod) group_left(node)
					kube_pod_info{$1}
				),
				"service",
				"$svc",
				"",
				""
			)
		)
	),
	1
)`,

	"meter_service_net_bytes_transmitted": `
round(
	sum by (namespace, service) (
		label_replace(
			sum by (namespace, pod) (
				sum by (namespace, pod) (
					sum_over_time(
						increase(
							container_network_transmit_bytes_total{
								pod!="",
								interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)", 
								job="kubelet"
							}[1h]
						)[$step:1h]
					)
				) * on (namespace, pod) group_left(owner_kind, owner_name)
				kube_pod_owner{} * on (namespace, pod) group_left(node)
				kube_pod_info{$1}
			),
			"service",
			"$svc",
			"",
			""
		)
	),
	1
)`,

	"meter_service_net_bytes_received": `
round(
	label_replace(
		sum by (namepace, pod) (
			sum by (namespace, pod) (
				sum_over_time(
					increase(
						container_network_receive_bytes_total{
							pod!="",
							interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)",
							job="kubelet"
						}[1h]
					)[$step:1h]
				)
			) * on (namespace, pod) group_left(owner_kind, owner_name)
			kube_pod_owner{} * on (namespace, pod) group_left(node)
			kube_pod_info{$1}
		),
		"service",
		"$svc",
		"",
		""
	),
	1
)`,

	// pod
	"meter_pod_cpu_usage": `
round(
	(
		sum by (namespace, pod) (
			sum_over_time(
				avg_over_time(
					namespace:kube_pod_resource_request:sum{
						owner_kind!="Job",
						resource="cpu",
					}[1h]
				)[$step:1h]
			)
		)
		* on (namespace, pod) group_left(owner_kind, owner_name)
		kube_pod_owner{$1}
		* on (namespace, pod) group_left(node)
		kube_pod_info{$2} >=
		sum by (namespace, pod) (
			sum_over_time(
				irate(container_cpu_usage_seconds_total{job="kubelet",pod!="",image!=""}[1h])[$step:1h]
			)
		)
		* on (namespace, pod) group_left(owner_kind, owner_name)
		kube_pod_owner{$1}
		* on (namespace, pod) group_left(node)
		kube_pod_info{$2}
	)
	or
	(
		sum by (namespace, pod) (
			sum_over_time(irate(container_cpu_usage_seconds_total{job="kubelet",pod!="",image!=""}[1h])[$step:1h])
		)
		* on (namespace, pod) group_left(owner_kind, owner_name)
		kube_pod_owner{$1}
		* on (namespace, pod) group_left(node)
		kube_pod_info{$2} >
		sum by (namespace, pod) (
			sum_over_time(
				avg_over_time(
					namespace:kube_pod_resource_request:sum{
						owner_kind!="Job",
						resource="cpu",
					}[1h]
				)[$step:1h]
			)
		)
		* on (namespace, pod) group_left(owner_kind, owner_name)
		kube_pod_owner{$1}
		* on (namespace, pod) group_left(node)
		kube_pod_info{$2}
	)
	or
	(
		sum by (namespace, pod) (
			sum_over_time(irate(container_cpu_usage_seconds_total{job="kubelet",pod!="",image!=""}[1h])[$step:1h])
		)
		* on (namespace, pod) group_left(owner_kind, owner_name)
		kube_pod_owner{$1}
		* on (namespace, pod) group_left(node)
		kube_pod_info{$2}
	),
	0.001
)`,

	"meter_pod_memory_usage_wo_cache": `
round(
	(
		sum by (namespace, pod) (
			sum_over_time(
				avg_over_time(
					namespace:kube_pod_resource_request:sum{
						owner_kind!="Job",
						resource="memory",
					}[1h]
				)[$step:1h]
			)
		)
		* on (namespace, pod) group_left(owner_kind, owner_name)
		kube_pod_owner{$1}
		* on (namespace, pod) group_left(node)
		kube_pod_info{$2} >=
		sum by (namespace, pod) (
			sum_over_time(
				avg_over_time(container_memory_working_set_bytes{job="kubelet", pod!="", image!=""}[1h])[$step:1h]
			)
		)
		* on (namespace, pod) group_left(owner_kind, owner_name)
		kube_pod_owner{$1}
		* on (namespace, pod) group_left(node)
		kube_pod_info{$2}
	)
	or
	(
		sum by (namespace, pod) (
			sum_over_time(
				avg_over_time(container_memory_working_set_bytes{job="kubelet", pod!="", image!=""}[1h])[$step:1h])
			)
			* on (namespace, pod) group_left(owner_kind, owner_name)
			kube_pod_owner{$1}
			* on (namespace, pod) group_left(node)
			kube_pod_info{$2} >
		sum by (namespace, pod) (
			sum_over_time(
				avg_over_time(
					namespace:kube_pod_resource_request:sum{
						owner_kind!="Job",
						resource="memory",
					}[1h]
				)[$step:1h]
			)
			* on (namespace, pod) group_left(owner_kind, owner_name)
			kube_pod_owner{$1}
			* on (namespace, pod) group_left(node)
			kube_pod_info{$2}
		)
	)
	or
	(
		sum by (namespace, pod) (
			sum_over_time(avg_over_time(container_memory_working_set_bytes{job="kubelet", pod!="", image!=""}[1h])[$step:1h])
		)
		* on (namespace, pod) group_left(owner_kind, owner_name)
		kube_pod_owner{$1}
		* on (namespace, pod) group_left(node)
		kube_pod_info{$2}
	),
	0.001
)`,

	"meter_pod_net_bytes_transmitted": `
sum by (namespace, pod) (
	sum_over_time(
		increase(
			container_network_transmit_bytes_total{
				pod!="", interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)", job="kubelet"
			}[1h]
		)[$step:1h]
	)
)
* on (namespace, pod) group_left(owner_kind, owner_name) kube_pod_owner{$1}
* on (namespace, pod) group_left(node) kube_pod_info{$2}`,

	"meter_pod_net_bytes_received": `
sum by (namespace, pod) (
	sum_over_time(
		increase(
			container_network_receive_bytes_total{
				pod!="", interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)", job="kubelet"
			}[1h]
		)[$step:1h]
	)
)
* on (namespace, pod) group_left(owner_kind, owner_name) kube_pod_owner{$1}
* on (namespace, pod) group_left(node) kube_pod_info{$2}`,

	"meter_pod_pvc_bytes_total": `
sum by (namespace, pod) (
	sum_over_time(avg_over_time(namespace:pvc_bytes_pod:sum{}[1h])[$step:1h])
)
* on (namespace, pod) group_left(owner_kind, owner_name) kube_pod_owner{$1}
* on (namespace, pod) group_left(node) kube_pod_info{$2}`,
}

func makeMeterExpr(meter string, o monitoring.QueryOptions) string {

	var tmpl string
	if tmpl = getMeterTemplate(meter); len(tmpl) == 0 {
		klog.Errorf("invalid meter %s", meter)
		return ""
	}
	tmpl = renderMeterTemplate(tmpl, o)

	switch o.Level {
	case monitoring.LevelCluster:
		return makeClusterMeterExpr(tmpl, o)
	case monitoring.LevelNode:
		return makeNodeMeterExpr(tmpl, o)
	case monitoring.LevelWorkspace:
		return makeWorkspaceMeterExpr(tmpl, o)
	case monitoring.LevelNamespace:
		return makeNamespaceMeterExpr(tmpl, o)
	case monitoring.LevelApplication:
		return makeApplicationMeterExpr(tmpl, o)
	case monitoring.LevelWorkload:
		return makeWorkloadMeterExpr(meter, tmpl, o)
	case monitoring.LevelService:
		return makeServiceMeterExpr(tmpl, o)
	case monitoring.LevelPod:
		return makePodMeterExpr(tmpl, o)
	default:
		return ""
	}

}

func getMeterTemplate(meter string) string {
	if tmpl, ok := promQLMeterTemplates[meter]; !ok {
		klog.Errorf("invalid meter %s", meter)
		return ""
	} else {
		return strings.Join(strings.Fields(strings.TrimSpace(tmpl)), " ")
	}
}

func renderMeterTemplate(tmpl string, o monitoring.QueryOptions) string {
	if o.MeterOptions == nil {
		klog.Error("meter options not found")
		return ""
	}

	tmpl = replaceStepSelector(tmpl, o)
	tmpl = replacePVCSelector(tmpl, o)
	tmpl = replaceNodeSelector(tmpl, o)
	tmpl = replaceInstanceSelector(tmpl, o)
	tmpl = replaceAppSelector(tmpl, o)
	tmpl = replaceSvcSelector(tmpl, o)

	return tmpl
}

func makeClusterMeterExpr(tmpl string, o monitoring.QueryOptions) string {
	return tmpl
}

func makeNodeMeterExpr(tmpl string, o monitoring.QueryOptions) string {
	return tmpl
}

func makeWorkspaceMeterExpr(tmpl string, o monitoring.QueryOptions) string {
	return makeWorkspaceMetricExpr(tmpl, o)
}

func makeNamespaceMeterExpr(tmpl string, o monitoring.QueryOptions) string {
	return makeNamespaceMetricExpr(tmpl, o)
}

func makeApplicationMeterExpr(tmpl string, o monitoring.QueryOptions) string {
	return strings.NewReplacer("$1", o.ResourceFilter).Replace(tmpl)
}

func makeWorkloadMeterExpr(meter string, tmpl string, o monitoring.QueryOptions) string {
	return makeWorkloadMetricExpr(meter, tmpl, o)
}

func makeServiceMeterExpr(tmpl string, o monitoring.QueryOptions) string {
	return strings.Replace(tmpl, "$1", o.ResourceFilter, -1)
}

func makePodMeterExpr(tmpl string, o monitoring.QueryOptions) string {
	return makePodMetricExpr(tmpl, o)
}

func replacePVCSelector(tmpl string, o monitoring.QueryOptions) string {
	var filterConditions []string

	switch o.Level {
	case monitoring.LevelCluster:
		break
	case monitoring.LevelNode:
		if o.NodeName != "" {
			filterConditions = append(filterConditions, fmt.Sprintf(`node="%s"`, o.NodeName))
		} else if o.ResourceFilter != "" {
			filterConditions = append(filterConditions, fmt.Sprintf(`node=~"%s"`, o.ResourceFilter))
		}
		if o.PVCFilter != "" {
			filterConditions = append(filterConditions, fmt.Sprintf(`persistentvolumeclaim=~"%s"`, o.PVCFilter))
		}
		if o.StorageClassName != "" {
			filterConditions = append(filterConditions, fmt.Sprintf(`storageclass="%s"`, o.StorageClassName))
		}
	case monitoring.LevelWorkspace:
		if o.WorkspaceName != "" {
			filterConditions = append(filterConditions, fmt.Sprintf(`workspace="%s"`, o.WorkspaceName))
		}
		if o.PVCFilter != "" {
			filterConditions = append(filterConditions, fmt.Sprintf(`persistentvolumeclaim=~"%s"`, o.PVCFilter))
		}
		if o.StorageClassName != "" {
			filterConditions = append(filterConditions, fmt.Sprintf(`storageclass="%s"`, o.StorageClassName))
		}
	case monitoring.LevelNamespace:
		if o.NamespaceName != "" {
			filterConditions = append(filterConditions, fmt.Sprintf(`namespace="%s"`, o.NamespaceName))
		}
		if o.PVCFilter != "" {
			filterConditions = append(filterConditions, fmt.Sprintf(`persistentvolumeclaim=~"%s"`, o.PVCFilter))
		}
		if o.StorageClassName != "" {
			filterConditions = append(filterConditions, fmt.Sprintf(`storageclass="%s"`, o.StorageClassName))
		}
	case monitoring.LevelApplication:
		if o.NamespaceName != "" {
			filterConditions = append(filterConditions, fmt.Sprintf(`namespace="%s"`, o.NamespaceName))
		}
		// o.PVCFilter is required and is filled with application volumes else just empty string which means that
		// there won't be any match application volumes
		filterConditions = append(filterConditions, fmt.Sprintf(`persistentvolumeclaim=~"%s"`, o.PVCFilter))

		if o.StorageClassName != "" {
			filterConditions = append(filterConditions, fmt.Sprintf(`storageclass="%s"`, o.StorageClassName))
		}
	default:
		return tmpl
	}
	return strings.Replace(tmpl, "$pvc", strings.Join(filterConditions, ","), -1)
}

func replaceStepSelector(tmpl string, o monitoring.QueryOptions) string {
	stepStr := strconv.Itoa(int(o.MeterOptions.Step.Hours())) + "h"

	return strings.Replace(tmpl, "$step", stepStr, -1)
}

func replaceNodeSelector(tmpl string, o monitoring.QueryOptions) string {

	var nodeSelector string
	if o.NodeName != "" {
		nodeSelector = fmt.Sprintf(`node="%s"`, o.NodeName)
	} else {
		nodeSelector = fmt.Sprintf(`node=~"%s"`, o.ResourceFilter)
	}
	return strings.Replace(tmpl, "$nodeSelector", nodeSelector, -1)
}

func replaceInstanceSelector(tmpl string, o monitoring.QueryOptions) string {
	var instanceSelector string
	if o.NodeName != "" {
		instanceSelector = fmt.Sprintf(`instance="%s"`, o.NodeName)
	} else {
		instanceSelector = fmt.Sprintf(`instance=~"%s"`, o.ResourceFilter)
	}
	return strings.Replace(tmpl, "$instanceSelector", instanceSelector, -1)
}

func replaceAppSelector(tmpl string, o monitoring.QueryOptions) string {
	return strings.Replace(tmpl, "$app", o.ApplicationName, -1)
}

func replaceSvcSelector(tmpl string, o monitoring.QueryOptions) string {
	return strings.Replace(tmpl, "$svc", o.ServiceName, -1)
}
