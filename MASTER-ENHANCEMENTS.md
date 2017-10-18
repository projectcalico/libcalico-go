# Enhancements in libcalico-go that need merging into the v3 integration branch

Here are the changes in master since the v3 integration branched off - with
most recent at the top:

	cc66874 *   Merge pull request #568 from fasaxc/fix-resync-loop
	        |\
	1b75146 | * Fix tight loop in KDD syncer if disableNodePoll enabled.
	        |/
	7320aec *   Merge pull request #566 from fasaxc/improve-port-msg
	        |\
	6586dfd | * Improve validation error message when ports are specified without a protocol.
	        |/
	4db56aa *   Merge pull request #515 from fasaxc/named-ports
	        |\
	dc54dc7 | * Add UTs for named ports and fix various marshalling and validation bugs.
	ca48bc0 | * Restore named ports feature.
	        |/
	8971c8a *   Merge pull request #500 from heschlie/node-etcd-leak
	        |\
	ea5e141 | * Tests for Node deletion
	        |/
	71101ce *   Merge pull request #526 from heschlie/watcher-leak
	        |\
	9659769 | * Fixing memory leak in KDD watchers
	        |/
	2fe92ae *   Merge pull request #543 from caseydavenport/master
	        |\
	d84059f | * Revert "Complete support for Calico policy using KDD client"
	        |/
	2da2134 *   Merge pull request #512 from song-jiang/host-endpoint-forward
	        |\
	cfd9eff | * Added ApplyOnForward flag to policy data model.
	5cabf71 * |   v1.7.1 Merge pull request #531 from caseydavenport/fix-defaulting
	        |\ \
	4e68ec2 | * | k8s conversion code handles 1.7 clusters
	        |/ /
	fb0f930 * |   v1.7.0 Merge pull request #523 from doublek/kdd-calico-policy
	        |\ \
	da68952 | * | Complete support for Calico policy using KDD client
	aeceb9b * | |   Merge pull request #524 from fasaxc/permissive-ingress-egress
	        |\ \ \
	23e4711 | * | | Make validation of policy types more permissive.
	        |/ / /
	6d51f61 * | |   Merge pull request #519 from fasaxc/back-out-named-ports
	        |\ \ \
	ea3386c | * | | Revert "Merge pull request #504 from fasaxc/named-ports"
	        | |/ /
	1417d07 * | |   Merge pull request #521 from bcreane/neil-egress-cidr
	        |\ \ \
	3aec1a6 | * | | Add support for k8s egress rules and CIDR blocks.
	        |/ / /
	d56c928 * | |   Merge pull request #520 from bcreane/cli-go
	        |\ \ \
	        | |/ /
	        |/| |
	639dbfa | * | Replace client-go with RESTClient for extensions/NetworkPolicy.
	        |/ /
	7eb7610 * |   v3-master-merge-base Merge pull request #514 from projectcalico/casey-reame-upd
