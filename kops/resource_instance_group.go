package kops

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/upup/pkg/fi/cloudup"
)

type instanceGroupID struct {
	clusterName       string
	instanceGroupName string
}

func (i instanceGroupID) String() string {
	return fmt.Sprintf("%s/%s", i.clusterName, i.instanceGroupName)
}

func parseInstanceGroupID(id string) instanceGroupID {
	split := strings.Split(id, "/")
	if len(split) == 2 {
		return instanceGroupID{
			clusterName:       split[0],
			instanceGroupName: split[1],
		}
	}
	return instanceGroupID{}
}

func resourceInstanceGroup() *schema.Resource {
	return &schema.Resource{
		Create: resourceInstanceGroupCreate,
		Read:   resourceInstanceGroupRead,
		Update: resourceInstanceGroupUpdate,
		Delete: resourceInstanceGroupDelete,
		Exists: resourceInstanceGroupExists,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"cluster_name": schemaStringRequired(),
			"metadata":     schemaMetadata(),
			"spec":         schemaInstanceGroupSpec(),
		},
	}
}

func resourceInstanceGroupCreate(d *schema.ResourceData, m interface{}) error {
	clusterName := d.Get("cluster_name").(string)
	clientset := m.(*ProviderConfig).clientset
	cluster, err := clientset.GetCluster(context.Background(), clusterName)
	if err != nil {
		return err
	}

	instanceGroup, err := clientset.InstanceGroupsFor(cluster).Create(
		context.Background(),
		&kops.InstanceGroup{
			ObjectMeta: expandObjectMeta(sectionData(d, "metadata")),
			Spec:       expandInstanceGroupSpec(sectionData(d, "spec")),
		},
		v1.CreateOptions{})
	if err != nil {
		return err
	}

	channel, err := cloudup.ChannelForCluster(cluster)
	if err != nil {
		return err
	}

	fullInstanceGroup, err := cloudup.PopulateInstanceGroupSpec(cluster, instanceGroup, channel)
	if err != nil {
		return err
	}

	_, err = clientset.InstanceGroupsFor(cluster).Update(context.Background(), fullInstanceGroup, v1.UpdateOptions{})
	if err != nil {
		return err
	}

	d.SetId(instanceGroupID{
		clusterName:       clusterName,
		instanceGroupName: instanceGroup.ObjectMeta.Name,
	}.String())

	return resourceInstanceGroupRead(d, m)
}

func resourceInstanceGroupRead(d *schema.ResourceData, m interface{}) error {
	instanceGroup, err := getInstanceGroup(d, m)
	if err != nil {
		return err
	}
	if err := d.Set("metadata", flattenObjectMeta(instanceGroup.ObjectMeta)); err != nil {
		return err
	}
	if err := d.Set("spec", flattenInstanceGroupSpec(instanceGroup.Spec)); err != nil {
		return err
	}
	return nil
}

func resourceInstanceGroupUpdate(d *schema.ResourceData, m interface{}) error {
	if ok, _ := resourceInstanceGroupExists(d, m); !ok {
		d.SetId("")
		return nil
	}

	clusterName := d.Get("cluster_name").(string)
	clientset := m.(*ProviderConfig).clientset
	cluster, err := clientset.GetCluster(context.Background(), clusterName)
	if err != nil {
		return err
	}

	_, err = clientset.InstanceGroupsFor(cluster).Update(
		context.Background(),
		&kops.InstanceGroup{
			ObjectMeta: expandObjectMeta(sectionData(d, "metadata")),
			Spec:       expandInstanceGroupSpec(sectionData(d, "spec")),
		},
		v1.UpdateOptions{})
	if err != nil {
		return err
	}

	return resourceInstanceGroupRead(d, m)
}

func resourceInstanceGroupDelete(d *schema.ResourceData, m interface{}) error {
	groupID := parseInstanceGroupID(d.Id())
	clientset := m.(*ProviderConfig).clientset
	cluster, err := clientset.GetCluster(context.Background(), groupID.clusterName)
	if err != nil {
		return err
	}
	return clientset.InstanceGroupsFor(cluster).Delete(context.Background(), groupID.instanceGroupName, v1.DeleteOptions{})
}

func resourceInstanceGroupExists(d *schema.ResourceData, m interface{}) (bool, error) {
	_, err := getInstanceGroup(d, m)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func getInstanceGroup(d *schema.ResourceData, m interface{}) (*kops.InstanceGroup, error) {
	groupID := parseInstanceGroupID(d.Id())
	clientset := m.(*ProviderConfig).clientset
	cluster, err := clientset.GetCluster(context.Background(), groupID.clusterName)
	if err != nil {
		return nil, err
	}
	return clientset.InstanceGroupsFor(cluster).Get(context.Background(), groupID.instanceGroupName, v1.GetOptions{})
}
