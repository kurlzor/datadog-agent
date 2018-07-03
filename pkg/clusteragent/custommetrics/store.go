// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2017 Datadog, Inc.

// +build kubeapiserver

package custommetrics

import (
	"encoding/json"
	"fmt"

	"github.com/DataDog/datadog-agent/pkg/util/log"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type CustomExternalMetric struct {
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels"`
	Timestamp int64             `json:"ts"`
	HpaName   string            `json:"hpa_name"`
	Value     int64             `json:"value"`
	Valid     bool              `json:"valid"`
}

type Store interface {
	UpdateExternalMetrics(updated []CustomExternalMetric) error
	DeleteExternalMetrics(metricNames []string) error
	ListAllExternalMetrics() ([]CustomExternalMetric, error)
}

type configMapStore struct {
	namespace string
	name      string
	client    corev1.CoreV1Interface
	cm        *v1.ConfigMap
}

// NewConfigMapStore returns a new store backed by a configmap. The configmap will be created
// in the specified namespace if it does not exist.
func NewConfigMapStore(client corev1.CoreV1Interface, ns, name string) (Store, error) {
	cm, err := client.ConfigMaps(ns).Get(name, metav1.GetOptions{})
	if err == nil {
		log.Infof("Retrieved the configmap %s", name)
		return &configMapStore{
			namespace: ns,
			name:      name,
			client:    client,
			cm:        cm,
		}, nil
	}

	if !errors.IsNotFound(err) {
		log.Infof("Error while attempting to fetch the configmap %s: %s", name, err)
		return nil, err
	}

	log.Infof("The configmap %s not dot exist, trying to create it", name)
	cm = &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}
	// FIXME: distinguish RBAC error
	cm, err = client.ConfigMaps(ns).Create(cm)
	if err != nil {
		return nil, err
	}
	return &configMapStore{
		namespace: ns,
		name:      name,
		client:    client,
		cm:        cm,
	}, nil
}

// UpdateExternalMetrics updates the specified metrics in the configmap. Only the leader replica
// should call this function.
func (c *configMapStore) UpdateExternalMetrics(updated []CustomExternalMetric) error {
	if c.cm == nil {
		return fmt.Errorf("configmap not initialized")
	}
	for _, metric := range updated {
		toStore, _ := json.Marshal(metric)
		if c.cm.Data == nil {
			// Don't panic "assignment to entry in nil map" at init
			c.cm.Data = make(map[string]string)
		}
		c.cm.Data[metric.Name] = string(toStore)
	}
	return c.updateConfigMap()
}

// DeleteExternalMetrics delete specified metrics from the configmap. Only the leader replica
// should call this function.
func (c *configMapStore) DeleteExternalMetrics(metricNames []string) error {
	if c.cm == nil {
		return fmt.Errorf("configmap not initialized")
	}
	for _, metricName := range metricNames {
		if c.cm.Data[metricName] != "" {
			delete(c.cm.Data, metricName)
			log.Debugf("Removed entry %#v from the configmap %s", metricName, c.name)
		}
	}
	return c.updateConfigMap()
}

// ListAllExternalMetrics returns the most up-to-date list of metrics from the configmap.
// Any replica can safely call this function.
func (c *configMapStore) ListAllExternalMetrics() ([]CustomExternalMetric, error) {
	var err error
	var metrics []CustomExternalMetric
	c.cm, err = c.client.ConfigMaps(c.namespace).Get(c.name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Could not store the custom metrics data in the configmap %s: %s", c.name, err.Error())
		return nil, nil
	}
	for _, d := range c.cm.Data {
		cm := &CustomExternalMetric{}
		json.Unmarshal([]byte(d), &cm)
		metrics = append(metrics, *cm)
	}
	return metrics, nil
}

func (c *configMapStore) updateConfigMap() error {
	var err error
	c.cm, err = c.client.ConfigMaps(c.namespace).Update(c.cm)
	if err != nil {
		log.Infof("Could not update the configmap: %s", err)
		return err
	}
	return nil
}
