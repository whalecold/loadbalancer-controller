package azure

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	log "github.com/zoumo/logdog"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/caicloud/clientset/informers"
	"github.com/caicloud/clientset/kubernetes"
	lblisters "github.com/caicloud/clientset/listers/loadbalance/v1alpha2"
	lbapi "github.com/caicloud/clientset/pkg/apis/loadbalance/v1alpha2"
	controllerutil "github.com/caicloud/clientset/util/controller"
	"github.com/caicloud/clientset/util/syncqueue"

	"github.com/caicloud/loadbalancer-controller/pkg/api"
	"github.com/caicloud/loadbalancer-controller/pkg/config"
	"github.com/caicloud/loadbalancer-controller/pkg/provider"
	"github.com/caicloud/loadbalancer-controller/pkg/toleration"
	lbutil "github.com/caicloud/loadbalancer-controller/pkg/util/lb"
)

const (
	providerNameSuffix = "-provider-azure"
	providerName       = "azure"
)

func init() {
	provider.RegisterPlugin(providerName, New())
}

var _ provider.Plugin = &azure{}

type azure struct {
	initialized bool
	image       string

	client kubernetes.Interface
	queue  *syncqueue.SyncQueue

	lbLister  lblisters.LoadBalancerLister
	dLister   appslisters.DeploymentLister
	podLister corelisters.PodLister
}

// New creates a new azure provider plugin
func New() provider.Plugin {
	return &azure{}
}

func (a *azure) Init(cfg config.Configuration, sif informers.SharedInformerFactory) {
	if a.initialized {
		return
	}
	a.initialized = true

	log.Info("Initialize the azure provider")

	// set config
	a.image = cfg.Providers.Azure.Image
	a.client = cfg.Client

	// initialize controller
	lbInformer := sif.Loadbalance().V1alpha2().LoadBalancers()
	dInformer := sif.Apps().V1().Deployments()
	podInfomer := sif.Core().V1().Pods()

	a.lbLister = lbInformer.Lister()
	a.dLister = dInformer.Lister()
	a.podLister = podInfomer.Lister()
	a.queue = syncqueue.NewPassthroughSyncQueue(&lbapi.LoadBalancer{}, a.syncLoadBalancer)

	dInformer.Informer().AddEventHandler(lbutil.NewEventHandlerForDeployment(a.lbLister, a.dLister, a.queue, a.deploymentFiltered))
	podInfomer.Informer().AddEventHandler(lbutil.NewEventHandlerForSyncStatusWithPod(a.lbLister, a.podLister, a.queue, a.podFiltered))
}

func (a *azure) Run(stopCh <-chan struct{}) {
	workers := 1

	if !a.initialized {
		log.Panic("Please initialize provider before you run it")
		return
	}

	defer utilruntime.HandleCrash()

	log.Info("Starting azure provider", log.Fields{"workers": workers})
	defer log.Info("Shutting down azure provider")

	// lb controller has waited all the informer synced
	// there is no need to wait again here

	defer func() {
		log.Info("Shutting down external provider")
		a.queue.ShutDown()
	}()

	a.queue.Run(workers)

	<-stopCh
}

func (a *azure) OnSync(lb *lbapi.LoadBalancer) {
	log.Info("Syncing azure providers, triggered by lb controller", log.Fields{"lb": lb.Name, "namespace": lb.Namespace})
	a.queue.Enqueue(lb)
}

func (a *azure) syncLoadBalancer(obj interface{}) error {
	lb, ok := obj.(*lbapi.LoadBalancer)
	if !ok {
		return fmt.Errorf("expect loadbalancer, got %v", obj)
	}

	// Validate loadbalancer scheme
	if err := lbapi.ValidateLoadBalancer(lb); err != nil {
		log.Debug("invalid loadbalancer scheme", log.Fields{"err": err})
		return err
	}

	key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(lb)

	startTime := time.Now()
	defer func() {
		log.Debug("Finished syncing azure provider", log.Fields{"lb": key, "usedTime": time.Since(startTime)})
	}()

	nlb, err := a.lbLister.LoadBalancers(lb.Namespace).Get(lb.Name)
	if errors.IsNotFound(err) {
		log.Warn("LoadBalancer has been deleted, clean up provider", log.Fields{"lb": key})

		return a.cleanup(lb, false)
	}
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to retrieve LoadBalancer %v from store: %v", key, err))
		return err
	}

	// fresh lb
	if lb.UID != nlb.UID {
		return nil
	}
	lb = nlb.DeepCopy()

	if lb.Spec.Providers.Azure == nil {
		// It is not my responsible, clean up legacies
		return a.cleanup(lb, true)
	}

	ds, err := a.getDeploymentsForLoadBalancer(lb)
	if err != nil {
		return err
	}

	if lb.DeletionTimestamp != nil {
		// TODO sync status only
		return nil
	}
	return a.sync(lb, ds)
}

func (a *azure) selector(lb *lbapi.LoadBalancer) labels.Set {
	return labels.Set{
		lbapi.LabelKeyCreatedBy: fmt.Sprintf(lbapi.LabelValueFormatCreateby, lb.Namespace, lb.Name),
		lbapi.LabelKeyProvider:  providerName,
	}
}

// cleanup deployment and other resource controlled by azure provider
func (a *azure) cleanup(lb *lbapi.LoadBalancer, deleteStatus bool) error {

	ds, err := a.getDeploymentsForLoadBalancer(lb)
	if err != nil {
		return err
	}

	policy := metav1.DeletePropagationForeground
	gracePeriodSeconds := int64(30)
	for _, d := range ds {
		a.client.AppsV1().Deployments(d.Namespace).Delete(d.Name, &metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriodSeconds,
			PropagationPolicy:  &policy,
		})
	}

	if deleteStatus {
		return a.deleteStatus(lb)
	}

	return nil
}

// filter Deployment that controller does not care
func (a *azure) deploymentFiltered(obj *appsv1.Deployment) bool {
	return a.filteredByLabel(obj)
}

func (a *azure) podFiltered(obj *v1.Pod) bool {
	return a.filteredByLabel(obj)
}

func (a *azure) filteredByLabel(obj metav1.ObjectMetaAccessor) bool {
	// obj.Labels
	selector := labels.Set{lbapi.LabelKeyProvider: providerName}.AsSelector()
	match := selector.Matches(labels.Set(obj.GetObjectMeta().GetLabels()))

	return !match
}

func (a *azure) getDeploymentsForLoadBalancer(lb *lbapi.LoadBalancer) ([]*appsv1.Deployment, error) {

	// construct selector
	selector := a.selector(lb).AsSelector()

	// list all
	dList, err := a.dLister.Deployments(lb.Namespace).List(selector)
	if err != nil {
		return nil, err
	}

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing deployment (see kubernetes#42639).
	canAdoptFunc := controllerutil.RecheckDeletionTimestamp(func() (metav1.Object, error) {
		// fresh lb
		fresh, err := a.client.LoadbalanceV1alpha2().LoadBalancers(lb.Namespace).Get(lb.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		if fresh.UID != lb.UID {
			return nil, fmt.Errorf("original LoadBalancer %v/%v is gone: got uid %v, wanted %v", lb.Namespace, lb.Name, fresh.UID, lb.UID)
		}
		return fresh, nil
	})

	cm := controllerutil.NewDeploymentControllerRefManager(a.client, lb, selector, api.ControllerKind, canAdoptFunc)
	return cm.Claim(dList)
}

// sync generate desired deployment from lb and compare it with existing deployment
func (a *azure) sync(lb *lbapi.LoadBalancer, dps []*appsv1.Deployment) error {
	desiredDeploy := a.generateDeployment(lb)

	// update
	updated := false
	activeDeploy := desiredDeploy

	for _, dp := range dps {
		// two conditions will trigger controller to scale down deployment
		// 1. deployment does not have auto-generated prefix
		// 2. if there are more than one active controllers, there may be many valid deployments.
		//    But we only need one.
		if !strings.HasPrefix(dp.Name, lb.Name+providerNameSuffix) || updated {
			if *dp.Spec.Replicas == 0 {
				continue
			}
			// scale unexpected deployment replicas to zero
			log.Info("Scale unexpected provider replicas to zero", log.Fields{"d.name": dp.Name, "lb.name": lb.Name})
			copy := dp.DeepCopy()
			replica := int32(0)
			copy.Spec.Replicas = &replica
			a.client.AppsV1().Deployments(lb.Namespace).Update(copy)
			continue
		}

		updated = true
		if lbutil.IsStatic(lb) {
			// do not change deployment if the loadbalancer is static
			activeDeploy = dp
		} else {
			copyDp, changed, err := a.ensureDeployment(desiredDeploy, dp)
			if err != nil {
				continue
			}
			if changed {
				log.Info("Sync azure for lb", log.Fields{"d.name": dp.Name, "lb.name": lb.Name})
				_, err = a.client.AppsV1().Deployments(lb.Namespace).Update(copyDp)
				if err != nil {
					return err
				}
			}

			activeDeploy = copyDp
		}
	}

	// len(dps) == 0 or no deployment's name match desired deployment
	if !updated {
		// create deployment
		log.Info("Create azure for lb", log.Fields{"d.name": desiredDeploy.Name, "lb.name": lb.Name})
		_, err := a.client.AppsV1().Deployments(lb.Namespace).Create(desiredDeploy)
		if err != nil {
			return err
		}
	}

	return a.syncStatus(lb, activeDeploy)
}

func (a *azure) generateDeployment(lb *lbapi.LoadBalancer) *appsv1.Deployment {
	terminationGracePeriodSeconds := int64(30)
	hostNetwork := true
	dnsPolicy := v1.DNSClusterFirstWithHostNet
	privileged := true
	maxSurge := intstr.FromInt(0)
	t := true
	replicas := int32(1)

	labels := a.selector(lb)

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   lb.Name + providerNameSuffix + "-" + lbutil.RandStringBytesRmndr(5),
			Labels: labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         api.ControllerKind.GroupVersion().String(),
					Kind:               api.ControllerKind.Kind,
					Name:               lb.Name,
					UID:                lb.UID,
					Controller:         &t,
					BlockOwnerDeletion: &t,
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge: &maxSurge,
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					// host network ?
					HostNetwork: hostNetwork,
					DNSPolicy:   dnsPolicy,
					// TODO
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					// tolerate taints
					Tolerations: toleration.GenerateTolerations(),
					Containers: []v1.Container{
						{
							Name:            providerName,
							Image:           a.image,
							ImagePullPolicy: v1.PullAlways,
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("200m"),
									v1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							SecurityContext: &v1.SecurityContext{
								Privileged: &privileged,
							},
							Env: []v1.EnvVar{
								{
									Name: "POD_NAME",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name:  "LOADBALANCER_NAMESPACE",
									Value: lb.Namespace,
								},
								{
									Name:  "LOADBALANCER_NAME",
									Value: lb.Name,
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "modules",
									MountPath: "/lib/modules",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "modules",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/lib/modules",
								},
							},
						},
					},
				},
			},
		},
	}

	return deploy
}

func (a *azure) ensureDeployment(desiredDeploy, oldDeploy *appsv1.Deployment) (*appsv1.Deployment, bool, error) {
	copyDp := oldDeploy.DeepCopy()

	// ensure labels
	for k, v := range desiredDeploy.Labels {
		copyDp.Labels[k] = v
	}
	// ensure replicas
	copyDp.Spec.Replicas = desiredDeploy.Spec.Replicas
	// ensure image
	copyDp.Spec.Template.Spec.Containers[0].Image = desiredDeploy.Spec.Template.Spec.Containers[0].Image
	// ensure nodeaffinity
	copyDp.Spec.Template.Spec.Affinity.NodeAffinity = desiredDeploy.Spec.Template.Spec.Affinity.NodeAffinity

	imageChanged := copyDp.Spec.Template.Spec.Containers[0].Image != oldDeploy.Spec.Template.Spec.Containers[0].Image
	labelChanged := !reflect.DeepEqual(copyDp.Labels, oldDeploy.Labels)
	replicasChanged := *(copyDp.Spec.Replicas) != *(oldDeploy.Spec.Replicas)

	changed := labelChanged || replicasChanged || imageChanged
	if changed {
		log.Info("Abount to correct azure provider", log.Fields{
			"dp.name":         copyDp.Name,
			"labelChanged":    labelChanged,
			"replicasChanged": replicasChanged,
			"imageChanged":    imageChanged,
		})
	}

	return copyDp, changed, nil
}
