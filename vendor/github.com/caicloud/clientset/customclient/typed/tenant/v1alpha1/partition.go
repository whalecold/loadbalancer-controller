/*
Copyright 2020 caicloud authors. All rights reserved.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"time"

	scheme "github.com/caicloud/clientset/customclient/scheme"
	v1alpha1 "github.com/caicloud/clientset/pkg/apis/tenant/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// PartitionsGetter has a method to return a PartitionInterface.
// A group's client should implement this interface.
type PartitionsGetter interface {
	Partitions() PartitionInterface
}

// PartitionInterface has methods to work with Partition resources.
type PartitionInterface interface {
	Create(*v1alpha1.Partition) (*v1alpha1.Partition, error)
	Update(*v1alpha1.Partition) (*v1alpha1.Partition, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.Partition, error)
	List(opts v1.ListOptions) (*v1alpha1.PartitionList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Partition, err error)
	PartitionExpansion
}

// partitions implements PartitionInterface
type partitions struct {
	client rest.Interface
}

// newPartitions returns a Partitions
func newPartitions(c *TenantV1alpha1Client) *partitions {
	return &partitions{
		client: c.RESTClient(),
	}
}

// Get takes name of the partition, and returns the corresponding partition object, and an error if there is any.
func (c *partitions) Get(name string, options v1.GetOptions) (result *v1alpha1.Partition, err error) {
	result = &v1alpha1.Partition{}
	err = c.client.Get().
		Resource("partitions").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Partitions that match those selectors.
func (c *partitions) List(opts v1.ListOptions) (result *v1alpha1.PartitionList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.PartitionList{}
	err = c.client.Get().
		Resource("partitions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested partitions.
func (c *partitions) Watch(opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("partitions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}

// Create takes the representation of a partition and creates it.  Returns the server's representation of the partition, and an error, if there is any.
func (c *partitions) Create(partition *v1alpha1.Partition) (result *v1alpha1.Partition, err error) {
	result = &v1alpha1.Partition{}
	err = c.client.Post().
		Resource("partitions").
		Body(partition).
		Do().
		Into(result)
	return
}

// Update takes the representation of a partition and updates it. Returns the server's representation of the partition, and an error, if there is any.
func (c *partitions) Update(partition *v1alpha1.Partition) (result *v1alpha1.Partition, err error) {
	result = &v1alpha1.Partition{}
	err = c.client.Put().
		Resource("partitions").
		Name(partition.Name).
		Body(partition).
		Do().
		Into(result)
	return
}

// Delete takes name of the partition and deletes it. Returns an error if one occurs.
func (c *partitions) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("partitions").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *partitions) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("partitions").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched partition.
func (c *partitions) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Partition, err error) {
	result = &v1alpha1.Partition{}
	err = c.client.Patch(pt).
		Resource("partitions").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}