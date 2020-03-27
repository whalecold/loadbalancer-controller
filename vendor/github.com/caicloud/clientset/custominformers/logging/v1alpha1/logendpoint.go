/*
Copyright 2020 caicloud authors. All rights reserved.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	time "time"

	customclient "github.com/caicloud/clientset/customclient"
	internalinterfaces "github.com/caicloud/clientset/custominformers/internalinterfaces"
	v1alpha1 "github.com/caicloud/clientset/listers/logging/v1alpha1"
	loggingv1alpha1 "github.com/caicloud/clientset/pkg/apis/logging/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// LogEndpointInformer provides access to a shared informer and lister for
// LogEndpoints.
type LogEndpointInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.LogEndpointLister
}

type logEndpointInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewLogEndpointInformer constructs a new informer for LogEndpoint type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewLogEndpointInformer(client customclient.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredLogEndpointInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredLogEndpointInformer constructs a new informer for LogEndpoint type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredLogEndpointInformer(client customclient.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.LoggingV1alpha1().LogEndpoints().List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.LoggingV1alpha1().LogEndpoints().Watch(options)
			},
		},
		&loggingv1alpha1.LogEndpoint{},
		resyncPeriod,
		indexers,
	)
}

func (f *logEndpointInformer) defaultInformer(client customclient.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredLogEndpointInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *logEndpointInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&loggingv1alpha1.LogEndpoint{}, f.defaultInformer)
}

func (f *logEndpointInformer) Lister() v1alpha1.LogEndpointLister {
	return v1alpha1.NewLogEndpointLister(f.Informer().GetIndexer())
}