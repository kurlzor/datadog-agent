// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2017 Datadog, Inc.

// +build kubeapiserver

package hpa

import (
	"reflect"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/zorkian/go-datadog-api.v2"
	"k8s.io/api/autoscaling/v2beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/DataDog/datadog-agent/pkg/clusteragent/custommetrics"
	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/util/kubernetes/apiserver/leaderelection"
	"github.com/DataDog/datadog-agent/pkg/util/log"
)

var (
	expectedHPAType = reflect.TypeOf(v2beta1.HorizontalPodAutoscaler{})
)

// HPAWatcherClient embeds the API Server client and the configuration to refresh metrics from Datadog and watch the HPA Objects' activities
type HPAWatcherClient struct {
	clientSet      kubernetes.Interface
	readTimeout    time.Duration
	refreshItl     *time.Ticker
	pollItl        *time.Ticker
	externalMaxAge time.Duration
	datadogClient  *datadog.Client
	store          custommetrics.Store
}

// NewHPAWatcherClient returns a new HPAWatcherClient
func NewHPAWatcherClient(clientSet kubernetes.Interface, store custommetrics.Store) (*HPAWatcherClient, error) {
	datadogCl, err := NewDatadogClient()
	if err != nil {
		return nil, err
	}
	pollInterval := config.Datadog.GetInt("hpa_watcher_polling_freq")
	refreshInterval := config.Datadog.GetInt("hpa_external_metrics_polling_freq")
	externalMaxAge := config.Datadog.GetInt("hpa_external_metrics_max_age")
	return &HPAWatcherClient{
		clientSet:      clientSet,
		readTimeout:    100 * time.Millisecond,
		pollItl:        time.NewTicker(time.Duration(pollInterval) * time.Second),
		refreshItl:     time.NewTicker(time.Duration(refreshInterval) * time.Second),
		externalMaxAge: time.Duration(externalMaxAge) * time.Second,
		datadogClient:  datadogCl,
		store:          store,
	}, nil
}

func (c *HPAWatcherClient) run(res string) (new []*v2beta1.HorizontalPodAutoscaler, modified []*v2beta1.HorizontalPodAutoscaler, deleted []*v2beta1.HorizontalPodAutoscaler, resVer string, err error) {
	metaOptions := metav1.ListOptions{Watch: true, ResourceVersion: res}
	watcher, err := c.clientSet.AutoscalingV2beta1().HorizontalPodAutoscalers(metav1.NamespaceAll).Watch(metaOptions)
	if err != nil {
		log.Infof("Failed to watch %v: %v", expectedHPAType, err)
	}
	defer watcher.Stop()

	watcherTimeout := time.NewTimer(c.readTimeout)
	for {
		select {
		case rcvdHPA, ok := <-watcher.ResultChan():
			if !ok {
				log.Debugf("Unexpected watch close")
				return nil, nil, nil, "0", err
			}
			currHPA, ok := rcvdHPA.Object.(*v2beta1.HorizontalPodAutoscaler)
			if !ok {
				log.Infof("Wrong type: %s", currHPA)
				continue
			}
			if currHPA.ResourceVersion != "" && currHPA.ResourceVersion != resVer {
				resVer = currHPA.ResourceVersion
			}
			if rcvdHPA.Type == watch.Error {
				status, ok := rcvdHPA.Object.(*metav1.Status)
				if !ok {
					return nil, nil, nil, "0", errors.Errorf("error in the watcher, evaluating: %s", currHPA)
				}
				log.Infof("Error while processing the HPA watch: %#v", status)
				continue
			}
			if rcvdHPA.Type == watch.Added {
				log.Debugf("Adding this manifest: %s", currHPA)
				new = append(new, currHPA)
			}
			if rcvdHPA.Type == watch.Modified {
				log.Debugf("Modifying this manifest: %s", currHPA)
				modified = append(modified, currHPA)
			}
			if rcvdHPA.Type == watch.Deleted {
				deleted = append(deleted, currHPA)
			}

			watcherTimeout.Reset(c.readTimeout)
		case <-watcherTimeout.C:
			return new, modified, deleted, resVer, nil
		}
	}
}

// Start runs a watch process of the various HPA objects' activities to process and store the relevant info.
// Refreshes the custom metrics stored as well.
func (c *HPAWatcherClient) Start() {
	log.Info("Starting HPA Process ...")
	tickerHPAWatchProcess := c.pollItl
	tickerHPARefreshProcess := c.refreshItl

	// Creating a leader election engine to make sure only the leader writes the metrics in the configmap and queries Datadog.
	leaderEngine, err := leaderelection.GetLeaderEngine()
	if err != nil {
		log.Errorf("Could not ensure the leader election is running properly: %s", err)
		return
	}
	leaderEngine.EnsureLeaderElectionRuns()

	var resversion string

	go func() {
		for {
			select {
			// Ticker for the HPA Object watcher
			case <-tickerHPAWatchProcess.C:
				if !leaderEngine.IsLeader() {
					continue
				}
				added, modified, deleted, res, err := c.run(resversion)
				if err != nil {
					log.Errorf("Error while watching HPA Objects' activities: %s", err)
					return
				}
				if res != resversion && res != "" {
					resversion = res
					if len(added) > 0 {
						c.processHPA(added)
					}
					if len(modified) > 0 {
						c.processHPA(modified)
					}
					if len(deleted) > 0 {
						log.Infof("deleting if resver is diff")
						c.removeEntryFromStore(deleted)
					}
				}
			// Ticker to run the refresh process for the stored external metrics
			case <-tickerHPARefreshProcess.C:
				if !leaderEngine.IsLeader() {
					continue
				}
				// Updating the metrics against Datadog should not affect the HPA pipeline.
				// If metrics are temporarily unavailable for too long, they will become `Valid=false` and won't be evaluated.
				c.updateExternalMetrics()
			}
		}
	}()
}

func (c *HPAWatcherClient) updateExternalMetrics() {
	maxAge := int64(c.externalMaxAge.Seconds())

	cmList, err := c.store.ListAllExternalMetrics()
	if err != nil {
		log.Infof("Error while retrieving external metrics from the store: %s", err)
		return
	}

	if len(cmList) == 0 {
		log.Debugf("No External Metrics to evaluate at the moment")
		return
	}

	for _, cm := range cmList {
		if metav1.Now().Unix()-cm.Timestamp <= maxAge && cm.Valid {
			continue
		}

		cm.Valid = false
		cm.Timestamp = metav1.Now().Unix()

		cm.Value, cm.Valid, err = c.validate(cm)
		if err != nil {
			log.Debugf("Could not update the metric %s from Datadog: %s", cm.Name, err.Error())
			continue
		}

		log.Debugf("Updated the custom metric %#v", cm)
	}

	if err := c.store.UpdateExternalMetrics(cmList); err != nil {
		log.Errorf("Could not store the custom metrics in the store: %s", err.Error())
	}
}

// processHPA transforms HPA data into structures to be stored upon validation that they are available in Datadog
// TODO Distinguish custom and external
func (hpa *HPAWatcherClient) processHPA(list []*v2beta1.HorizontalPodAutoscaler) error {
	var cmList []custommetrics.CustomExternalMetric
	var err error
	for _, e := range list {
		for _, m := range e.Spec.Metrics {
			var cm custommetrics.CustomExternalMetric
			cm.Name = m.External.MetricName
			cm.Timestamp = metav1.Now().Unix()
			cm.Labels = m.External.MetricSelector.MatchLabels
			cm.HpaName = e.Name
			cm.Value, cm.Valid, err = hpa.validate(cm)
			if err != nil {
				log.Debugf("Not able to process %#v: %s", cm, err)
			}
			cmList = append(cmList, cm)
		}
	}
	if err := hpa.store.UpdateExternalMetrics(cmList); err != nil {
		log.Infof("Could not update external metrics in the store: %s", err.Error())
		return err
	}
	return nil
}

// validate queries Datadog to validate the availability of a metric
func (hpa *HPAWatcherClient) validate(cm custommetrics.CustomExternalMetric) (value int64, valid bool, err error) {
	val, err := hpa.queryDatadogExternal(cm.Name, cm.Labels)
	if err != nil {
		return cm.Value, false, err
	}
	cm.Valid = true
	return val, true, nil
}

// removeEntryFromStore will remove an External Custom Metric from removeEntryFromStore if the corresponding HPA manifest is deleted.
func (c *HPAWatcherClient) removeEntryFromStore(deleted []*v2beta1.HorizontalPodAutoscaler) error {
	metricNames := make([]string, 0)
	for _, d := range deleted {
		for _, m := range d.Spec.Metrics {
			metricNames = append(metricNames, m.External.MetricName)
		}
	}
	if err := c.store.DeleteExternalMetrics(metricNames); err != nil {
		log.Infof("Could not delete external metrics in the store: %s", err.Error())
		return err
	}
	return nil
}

// Stop sends a signal to the HPAWatcher to stop it.
// Used for the tests to avoid leaking go-routines.
func (c *HPAWatcherClient) Stop() {
	c.pollItl.Stop()
	c.refreshItl.Stop()
}
