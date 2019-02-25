package azure

import (
	log "github.com/zoumo/logdog"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lbapi "github.com/caicloud/clientset/pkg/apis/loadbalance/v1alpha2"

	lbutil "github.com/caicloud/loadbalancer-controller/pkg/util/lb"
	stringsutil "github.com/caicloud/loadbalancer-controller/pkg/util/strings"
)

func (a *azure) syncStatus(lb *lbapi.LoadBalancer, activeDeploy *appsv1.Deployment) error {
	if lb.Spec.Providers.Azure == nil {
		return a.deleteStatus(lb)
	}
	// caculate proxy status
	return nil
}

func (a *azure) deleteStatus(lb *lbapi.LoadBalancer) error {
	if lb.Status.ProvidersStatuses.Azure == nil {
		return nil
	}

	log.Notice("delete azure status", log.Fields{"lb.name": lb.Name, "lb.ns": lb.Namespace})
	_, err := lbutil.UpdateLBWithRetries(
		a.client.LoadbalanceV1alpha2().LoadBalancers(lb.Namespace),
		a.lbLister,
		lb.Namespace,
		lb.Name,
		func(lb *lbapi.LoadBalancer) error {
			lb.Status.ProvidersStatuses.Azure = nil
			return nil
		},
	)

	if err != nil {
		log.Error("Update loadbalancer status error", log.Fields{"err": err})
		return err
	}
	return nil
}

func (a *azure) evictPod(lb *lbapi.LoadBalancer, pod *v1.Pod) {

	if len(lb.Spec.Nodes.Names) == 0 {
		return
	}

	// fix: avoid evict pending pod
	if pod.Spec.NodeName == "" {
		return
	}

	// FIXME: when RequiredDuringSchedulingRequiredDuringExecution finished
	// This is a special issue.
	// There is bug when the nodes.Names change。
	// According to nodeAffinity RequiredDuringSchedulingIgnoredDuringExecution,
	// the system may or may not try to eventually evict the pod from its node.
	// the pod may still running on the wrong node, so we evict it manually
	if !stringsutil.StringInSlice(pod.Spec.NodeName, lb.Spec.Nodes.Names) &&
		pod.DeletionTimestamp == nil {
		a.client.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{})
	}
}
