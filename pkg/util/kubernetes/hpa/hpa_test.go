// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2017 Datadog, Inc.

// +build kubeapiserver

package hpa

import (
	"fmt"
	"testing"

	"github.com/DataDog/datadog-agent/pkg/clusteragent/custommetrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/api/autoscaling/v2beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type mockStore struct {
	metrics map[string]custommetrics.CustomExternalMetric
}

func newMockStore(metricName string, labels map[string]string) *mockStore {
	s := &mockStore{}
	cm := custommetrics.CustomExternalMetric{
		Name:      metricName,
		Labels:    labels,
		Timestamp: 12,
		HpaName:   "foo",
		Value:     1,
		Valid:     false,
	}
	s.UpdateExternalMetrics([]custommetrics.CustomExternalMetric{cm})
	return s
}

func (s *mockStore) UpdateExternalMetrics(updated []custommetrics.CustomExternalMetric) error {
	for _, cm := range updated {
		if s.metrics == nil {
			s.metrics = make(map[string]custommetrics.CustomExternalMetric)
		}
		s.metrics[cm.Name] = cm
	}
	return nil
}

func (s *mockStore) DeleteExternalMetrics(metricNames []string) error {
	for _, metricName := range metricNames {
		delete(s.metrics, metricName)
	}
	return nil
}

func (s *mockStore) ListAllExternalMetrics() ([]custommetrics.CustomExternalMetric, error) {
	allMetrics := make([]custommetrics.CustomExternalMetric, 0)
	for _, cm := range s.metrics {
		allMetrics = append(allMetrics, cm)
	}
	return allMetrics, nil
}

func newMockHPAExternalManifest(metricName string, labels map[string]string) *v2beta1.HorizontalPodAutoscaler {
	return &v2beta1.HorizontalPodAutoscaler{
		Spec: v2beta1.HorizontalPodAutoscalerSpec{
			Metrics: []v2beta1.MetricSpec{
				{
					External: &v2beta1.ExternalMetricSource{
						MetricName: metricName,
						MetricSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
			},
		},
	}
}

func TestRemoveEntryFromStore(t *testing.T) {
	hpaCl := HPAWatcherClient{clientSet: fake.NewSimpleClientset()}

	testCases := []struct {
		caseName        string
		store           custommetrics.Store
		hpa             *v2beta1.HorizontalPodAutoscaler
		expectedMetrics map[string]custommetrics.CustomExternalMetric
	}{
		{
			caseName:        "Metric exists, deleting",
			store:           newMockStore("foo", map[string]string{"bar": "baz"}),
			hpa:             newMockHPAExternalManifest("foo", map[string]string{"bar": "baz"}),
			expectedMetrics: map[string]custommetrics.CustomExternalMetric{},
		},
		{
			caseName: "Metric is not listed, no-op",
			store:    newMockStore("foobar", map[string]string{"bar": "baz"}),
			hpa:      newMockHPAExternalManifest("foo", map[string]string{"bar": "baz"}),
			expectedMetrics: map[string]custommetrics.CustomExternalMetric{
				"foobar": custommetrics.CustomExternalMetric{
					Name:      "foobar",
					Labels:    map[string]string{"bar": "baz"},
					Timestamp: 12,
					HpaName:   "foo",
					Value:     1,
					Valid:     false,
				},
			},
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("#%d %s", i, testCase.caseName), func(t *testing.T) {
			hpaCl.store = testCase.store
			require.NotZero(t, len(hpaCl.store.(*mockStore).metrics))

			err := hpaCl.removeEntryFromStore([]*v2beta1.HorizontalPodAutoscaler{testCase.hpa})
			require.NoError(t, err)
			assert.Equal(t, testCase.expectedMetrics, hpaCl.store.(*mockStore).metrics)
		})
	}
}
