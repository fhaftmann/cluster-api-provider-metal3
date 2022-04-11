/*
Copyright 2019 The Kubernetes Authors.
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

package baremetal

import (
	"context"
	"time"

	"github.com/go-logr/logr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type testCaseRemediationManager struct {
	Metal3Remediation *infrav1.Metal3Remediation
	Metal3Machine     *infrav1.Metal3Machine
	Machine           *clusterv1.Machine
	ExpectSuccess     bool
}

var _ = Describe("Metal3Remediation manager", func() {

	var fakeClient client.Client

	BeforeEach(func() {
		fakeClient = fake.NewClientBuilder().WithScheme(setupScheme()).Build()
	})

	Describe("Test New Remediation Manager", func() {

		DescribeTable("Test NewRemediationManager",
			func(tc testCaseRemediationManager) {
				_, err := NewRemediationManager(fakeClient,
					tc.Metal3Remediation,
					tc.Metal3Machine,
					tc.Machine,
					logr.Discard(),
				)
				if tc.ExpectSuccess {
					Expect(err).NotTo(HaveOccurred())
				} else {
					Expect(err).To(HaveOccurred())
				}
			},
			Entry("All fields defined", testCaseRemediationManager{
				Metal3Remediation: &infrav1.Metal3Remediation{},
				Metal3Machine:     &infrav1.Metal3Machine{},
				Machine:           &clusterv1.Machine{},
				ExpectSuccess:     true,
			}),
			Entry("None of the fields defined", testCaseRemediationManager{
				Metal3Remediation: nil,
				Metal3Machine:     nil,
				Machine:           nil,
				ExpectSuccess:     true,
			}),
		)
	})

	DescribeTable("Test Finalizers",
		func(tc testCaseRemediationManager) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			remediationMgr.SetFinalizer()

			Expect(tc.Metal3Remediation.ObjectMeta.Finalizers).To(ContainElement(
				infrav1.RemediationFinalizer,
			))

			remediationMgr.UnsetFinalizer()

			Expect(tc.Metal3Remediation.ObjectMeta.Finalizers).NotTo(ContainElement(
				infrav1.RemediationFinalizer,
			))
		},
		Entry("No finalizers", testCaseRemediationManager{
			Metal3Remediation: &infrav1.Metal3Remediation{},
		}),
		Entry("Additional finalizers", testCaseRemediationManager{
			Metal3Remediation: &infrav1.Metal3Remediation{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"foo"},
				},
			},
		}),
	)

	type testCaseRetryLimitSet struct {
		Metal3Remediation *infrav1.Metal3Remediation
		ExpectTrue        bool
	}

	DescribeTable("Test if Retry Limit is set",
		func(tc testCaseRetryLimitSet) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			retryLimitIsSet := remediationMgr.RetryLimitIsSet()

			Expect(retryLimitIsSet).To(Equal(tc.ExpectTrue))
		},
		Entry("retry limit is set", testCaseRetryLimitSet{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       "",
						RetryLimit: 1,
						Timeout:    &metav1.Duration{},
					},
				},
			},
			ExpectTrue: true,
		}),
		Entry("retry limit is set to 0", testCaseRetryLimitSet{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       "",
						RetryLimit: 0,
						Timeout:    &metav1.Duration{},
					},
				},
			},
			ExpectTrue: false,
		}),
		Entry("retry limit is set to less than 0", testCaseRetryLimitSet{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       "",
						RetryLimit: -1,
						Timeout:    &metav1.Duration{},
					},
				},
			},
			ExpectTrue: false,
		}),
		Entry("retry limit is not set", testCaseRetryLimitSet{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:    "",
						Timeout: &metav1.Duration{},
					},
				},
			},
			ExpectTrue: false,
		}),
	)

	DescribeTable("Test if Retry Limit is reached",
		func(tc testCaseRetryLimitSet) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			retryLimitIsSet := remediationMgr.HasReachRetryLimit()

			Expect(retryLimitIsSet).To(Equal(tc.ExpectTrue))
		},
		Entry("retry limit is reached", testCaseRetryLimitSet{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       "",
						RetryLimit: 1,
						Timeout:    &metav1.Duration{},
					},
				},
				Status: infrav1.Metal3RemediationStatus{
					Phase:          "",
					RetryCount:     1,
					LastRemediated: &metav1.Time{},
				},
			},
			ExpectTrue: true,
		}),
		Entry("retry limit is not reached", testCaseRetryLimitSet{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       "",
						RetryLimit: 1,
						Timeout:    &metav1.Duration{},
					},
				},
				Status: infrav1.Metal3RemediationStatus{
					Phase:          "",
					RetryCount:     0,
					LastRemediated: &metav1.Time{},
				},
			},
			ExpectTrue: false,
		}),
		Entry("retry limit is not set so limit is reached", testCaseRetryLimitSet{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       "",
						RetryLimit: 0,
						Timeout:    &metav1.Duration{},
					},
				},
				Status: infrav1.Metal3RemediationStatus{
					Phase:          "",
					RetryCount:     0,
					LastRemediated: &metav1.Time{},
				},
			},
			ExpectTrue: true,
		}),
	)

	type testCaseEnsureRebootAnnotation struct {
		Host              *bmov1alpha1.BareMetalHost
		Metal3Remediation *infrav1.Metal3Remediation
		ExpectTrue        bool
	}

	DescribeTable("Test OnlineStatus",
		func(tc testCaseEnsureRebootAnnotation) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			onlineStatus := remediationMgr.OnlineStatus(tc.Host)
			if tc.ExpectTrue {
				Expect(onlineStatus).To(BeTrue())
			} else {
				Expect(onlineStatus).To(BeFalse())
			}
		},
		Entry(" Online field in spec is set to false", testCaseEnsureRebootAnnotation{
			Host: &bmov1alpha1.BareMetalHost{
				Spec: bmov1alpha1.BareMetalHostSpec{
					Online: false,
				},
			},
			ExpectTrue: false,
		}),
		Entry(" Online field in spec is set to true", testCaseEnsureRebootAnnotation{
			Host: &bmov1alpha1.BareMetalHost{
				Spec: bmov1alpha1.BareMetalHostSpec{
					Online: true,
				},
			},
			ExpectTrue: true,
		}),
	)

	type testCaseGetUnhealthyHost struct {
		M3Machine         *infrav1.Metal3Machine
		Metal3Remediation *infrav1.Metal3Remediation
		ExpectPresent     bool
	}

	DescribeTable("Test GetUnhealthyHost",
		func(tc testCaseGetUnhealthyHost) {
			host := bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myhost",
					Namespace: namespaceName,
				},
			}

			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(&host).Build()

			remediationMgr, err := NewRemediationManager(fakeClient, nil, tc.M3Machine, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			result, helper, err := remediationMgr.GetUnhealthyHost(context.TODO())
			if tc.ExpectPresent {
				Expect(result).NotTo(BeNil())
				Expect(helper).NotTo(BeNil())
				Expect(err).To(BeNil())
			} else {
				Expect(result).To(BeNil())
				Expect(helper).To(BeNil())
				Expect(err).NotTo(BeNil())
			}
		},
		Entry("Should find the unhealthy host", testCaseGetUnhealthyHost{
			M3Machine: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       namespaceName,
					OwnerReferences: []metav1.OwnerReference{},
					Annotations: map[string]string{
						HostAnnotation: namespaceName + "/myhost",
					},
				},
			},
			ExpectPresent: true,
		}),
		Entry("Should not find the unhealthy host", testCaseGetUnhealthyHost{
			M3Machine: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       namespaceName,
					OwnerReferences: []metav1.OwnerReference{},
					Annotations: map[string]string{
						HostAnnotation: namespaceName + "/wronghostname",
					},
				},
			},
			ExpectPresent: false,
		}),
		Entry("Should not find the host, annotation is empty", testCaseGetUnhealthyHost{
			M3Machine: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       namespaceName,
					OwnerReferences: []metav1.OwnerReference{},
					Annotations:     map[string]string{},
				},
			},
			ExpectPresent: false,
		}),
		Entry("Should not find the host, annotation is nil", testCaseGetUnhealthyHost{
			M3Machine: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       "myns",
					OwnerReferences: []metav1.OwnerReference{},
					Annotations:     nil,
				},
			},
			ExpectPresent: false,
		}),
		Entry("Should not find the host, could not parse annotation value", testCaseGetUnhealthyHost{
			M3Machine: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       "myns",
					OwnerReferences: []metav1.OwnerReference{},
					Annotations: map[string]string{
						HostAnnotation: "myns/wronghostname/wronghostname",
					},
				},
			},
			ExpectPresent: false,
		}),
	)

	type testCaseSetAnnotation struct {
		Host       *bmov1alpha1.BareMetalHost
		M3Machine  *infrav1.Metal3Machine
		ExpectTrue bool
	}

	DescribeTable("Test SetUnhealthyAnnotation",
		func(tc testCaseSetAnnotation) {
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(tc.Host).Build()
			remediationMgr, err := NewRemediationManager(fakeClient, nil, tc.M3Machine, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			patchError := remediationMgr.SetUnhealthyAnnotation(context.TODO())

			if tc.ExpectTrue {
				Expect(patchError).To(BeNil())
			} else {
				Expect(patchError).NotTo(BeNil())
			}
		},
		Entry("Should set the unhealthy annotation", testCaseSetAnnotation{
			M3Machine: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       "myns",
					OwnerReferences: []metav1.OwnerReference{},
					Annotations: map[string]string{
						HostAnnotation: "myns/myhost",
					},
				},
			},
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "myhost",
					Namespace:   "myns",
					Annotations: map[string]string{infrav1.UnhealthyAnnotation: ""},
				},
			},
			ExpectTrue: true,
		}),
		Entry("Should not set the unhealthy annotation, annotation is empty", testCaseSetAnnotation{
			M3Machine: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       "myns",
					OwnerReferences: []metav1.OwnerReference{},
					Annotations:     map[string]string{},
				},
			},
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "myhost",
					Namespace:   "myns",
					Annotations: map[string]string{infrav1.UnhealthyAnnotation: ""},
				},
			},
			ExpectTrue: false,
		}),
		Entry("Should not set the unhealthy annotation because of wrong HostAnnotation", testCaseSetAnnotation{
			M3Machine: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       "myns",
					OwnerReferences: []metav1.OwnerReference{},
					Annotations: map[string]string{
						HostAnnotation: "myns/wronghostname",
					},
				},
			},
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "myhost",
					Namespace:   "myns",
					Annotations: map[string]string{infrav1.UnhealthyAnnotation: ""},
				},
			},
			ExpectTrue: false,
		}),
	)

	DescribeTable("Test SetRebootAnnotation",
		func(tc testCaseSetAnnotation) {
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(tc.Host).Build()
			remediationMgr, err := NewRemediationManager(fakeClient, nil, tc.M3Machine, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			patchError := remediationMgr.SetRebootAnnotation(context.TODO())

			if tc.ExpectTrue {
				Expect(patchError).To(BeNil())
			} else {
				Expect(patchError).NotTo(BeNil())
			}
		},
		Entry("Should set the reboot annotation", testCaseSetAnnotation{
			M3Machine: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       "myns",
					OwnerReferences: []metav1.OwnerReference{},
					Annotations: map[string]string{
						HostAnnotation: "myns/myhost",
					},
				},
			},
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "myhost",
					Namespace:   "myns",
					Annotations: map[string]string{rebootAnnotation: ""},
				},
			},
			ExpectTrue: true,
		}),
		Entry("Should not set the reboot annotation, annotation is empty", testCaseSetAnnotation{
			M3Machine: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       "myns",
					OwnerReferences: []metav1.OwnerReference{},
					Annotations:     map[string]string{},
				},
			},
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "myhost",
					Namespace:   "myns",
					Annotations: map[string]string{rebootAnnotation: ""},
				},
			},
			ExpectTrue: false,
		}),
		Entry("Should not set the reboot annotation because of wrong HostAnnotation", testCaseSetAnnotation{
			M3Machine: &infrav1.Metal3Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mym3machine",
					Namespace:       "myns",
					OwnerReferences: []metav1.OwnerReference{},
					Annotations: map[string]string{
						HostAnnotation: "myns/wronghostname",
					},
				},
			},
			Host: &bmov1alpha1.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "myhost",
					Namespace:   "myns",
					Annotations: map[string]string{rebootAnnotation: ""},
				},
			},
			ExpectTrue: false,
		}),
	)

	type testCaseGetRemediationType struct {
		Metal3Remediation  *infrav1.Metal3Remediation
		RemediationType    *infrav1.RemediationType
		RemediationTypeSet bool
	}

	DescribeTable("Test GetRemediationType",
		func(tc testCaseGetRemediationType) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			remediationType := remediationMgr.GetRemediationType()
			if tc.RemediationTypeSet {
				Expect(remediationType).To(Equal(infrav1.RebootRemediationStrategy))
			} else {
				Expect(remediationType).To(Equal(infrav1.RemediationType("")))
			}
		},
		Entry("Remediation strategy type is set to Reboot, should return strategy", testCaseGetRemediationType{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       "Reboot",
						RetryLimit: 0,
						Timeout:    &metav1.Duration{},
					},
				},
			},
			RemediationTypeSet: true,
		}),
		Entry("Remediation strategy type is not set should return empty string", testCaseGetRemediationType{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       "",
						RetryLimit: 0,
						Timeout:    &metav1.Duration{},
					},
				},
			},
			RemediationTypeSet: false,
		}),
	)

	type testCaseGetRemediatedTime struct {
		Metal3Remediation *infrav1.Metal3Remediation
		Remediated        bool
	}

	DescribeTable("Test GetLastRemediatedTime",
		func(tc testCaseGetRemediatedTime) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			remediationTime := remediationMgr.GetLastRemediatedTime()
			if tc.Remediated {
				Expect(remediationTime).NotTo(BeNil())
			} else {
				Expect(remediationTime).To(BeNil())
			}
		},
		Entry("Host is not yet remediated and last remediation timestamp is not set yet", testCaseGetRemediatedTime{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Status: infrav1.Metal3RemediationStatus{},
			},
			Remediated: false,
		}),
		Entry("Host is remediated and controller has set the last remediation timestamp", testCaseGetRemediatedTime{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Status: infrav1.Metal3RemediationStatus{
					LastRemediated: &metav1.Time{},
				},
			},
			Remediated: true,
		}),
	)

	type testTimeToRemediate struct {
		Metal3Remediation *infrav1.Metal3Remediation
		ExpectTrue        bool
	}

	DescribeTable("Test TimeToRemediate",
		func(tc testTimeToRemediate) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			okToRemediate, nextRemediation := remediationMgr.TimeToRemediate(remediationMgr.GetTimeout().Duration)
			if tc.ExpectTrue {
				Expect(okToRemediate).To(BeTrue())
				Expect(nextRemediation).To(Equal(time.Duration(0)))

			} else {
				Expect(okToRemediate).To(BeFalse())
				Expect(nextRemediation).NotTo(Equal(time.Duration(0)))
			}

		},
		Entry("Time to remediate reached", testTimeToRemediate{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       "",
						RetryLimit: 1,
						Timeout:    &metav1.Duration{Duration: 600 * time.Second},
					},
				},
				Status: infrav1.Metal3RemediationStatus{
					Phase:          "",
					RetryCount:     1,
					LastRemediated: &metav1.Time{Time: time.Now().Add(time.Duration(-700) * time.Second)},
				},
			},
			ExpectTrue: true,
		}),
		Entry("Time to remediate is not reached because of LastRemediated is nil", testTimeToRemediate{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       "",
						RetryLimit: 1,
						Timeout:    &metav1.Duration{Duration: 600 * time.Second},
					},
				},
				Status: infrav1.Metal3RemediationStatus{
					Phase:          "",
					RetryCount:     1,
					LastRemediated: nil,
				},
			},
			ExpectTrue: false,
		}),
		Entry("Time to remediate is not reached, LastRemediated + Timeout is greater than current time", testTimeToRemediate{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       "",
						RetryLimit: 1,
						Timeout:    &metav1.Duration{Duration: 600 * time.Second},
					},
				},
				Status: infrav1.Metal3RemediationStatus{
					Phase:          "",
					RetryCount:     1,
					LastRemediated: &metav1.Time{Time: time.Now()},
				},
			},
			ExpectTrue: false,
		}),
	)

	type testCaseGetTimeout struct {
		Metal3Remediation *infrav1.Metal3Remediation
		TimeoutSet        bool
	}

	DescribeTable("Test GetTimeout",
		func(tc testCaseGetTimeout) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			timeout := remediationMgr.GetTimeout()
			if tc.TimeoutSet {
				Expect(timeout).NotTo(BeNil())
			} else {
				Expect(timeout).To(BeNil())
			}
		},
		Entry("Timeout is not set", testCaseGetTimeout{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{},
				},
			},
			TimeoutSet: false,
		}),
		Entry("Timeout is set", testCaseGetTimeout{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       "",
						RetryLimit: 0,
						Timeout:    &metav1.Duration{},
					},
				},
			},
			TimeoutSet: true,
		}),
	)

	DescribeTable("Test SetRemediationPhase",
		func(tc testCaseRemediationManager) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			remediationMgr.SetRemediationPhase(infrav1.PhaseRunning)

			Expect(tc.Metal3Remediation.Status.Phase).To(Equal("Running"))
		},
		Entry("No phase", testCaseRemediationManager{
			Metal3Remediation: &infrav1.Metal3Remediation{},
		}),
		Entry("Overwride excisting phase", testCaseRemediationManager{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Status: infrav1.Metal3RemediationStatus{
					Phase: "Waiting",
				},
			},
		}),
	)

	DescribeTable("Test SetLastRemediationTime",
		func(tc testCaseRemediationManager) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			now := metav1.Now()

			remediationMgr.SetLastRemediationTime(&now)

			Expect(tc.Metal3Remediation.Status.LastRemediated).ShouldNot(BeNil())
		},
		Entry("No timestamp set", testCaseRemediationManager{
			Metal3Remediation: &infrav1.Metal3Remediation{},
		}),
		Entry("Overwride excisting time", testCaseRemediationManager{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Status: infrav1.Metal3RemediationStatus{
					LastRemediated: &metav1.Time{},
				},
			},
		}),
	)

	DescribeTable("Test IncreaseRetryCount",
		func(tc testCaseRemediationManager) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			oldCount := tc.Metal3Remediation.Status.RetryCount

			remediationMgr.IncreaseRetryCount()
			newCount := oldCount + 1
			Expect(tc.Metal3Remediation.Status.RetryCount).To(Equal(newCount))
		},
		Entry("RetryCount is not set", testCaseRemediationManager{
			Metal3Remediation: &infrav1.Metal3Remediation{},
		}),
		Entry("RetryCount is 0", testCaseRemediationManager{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Status: infrav1.Metal3RemediationStatus{
					RetryCount: 0,
				},
			},
		}),
		Entry("RetryCount is 2", testCaseRemediationManager{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Status: infrav1.Metal3RemediationStatus{
					RetryCount: 2,
				},
			},
		}),
	)

	type testCaseGetRemediationPhase struct {
		Metal3Remediation *infrav1.Metal3Remediation
		Succeed           bool
	}

	DescribeTable("Test GetRemediationPhase",
		func(tc testCaseGetRemediationPhase) {
			remediationMgr, err := NewRemediationManager(nil, tc.Metal3Remediation, nil, nil,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			phase := remediationMgr.GetRemediationPhase()

			if tc.Succeed {
				if phase == infrav1.PhaseRunning {
					Expect(tc.Metal3Remediation.Status.Phase).Should(ContainSubstring("Running"))
				}
				if phase == infrav1.PhaseWaiting {
					Expect(tc.Metal3Remediation.Status.Phase).Should(ContainSubstring("Waiting"))
				}
				if phase == infrav1.PhaseDeleting {
					Expect(tc.Metal3Remediation.Status.Phase).Should(ContainSubstring("Deleting machine"))
				}
			} else {
				Expect(phase).ShouldNot(ContainSubstring("Deleting machine"))
				Expect(phase).ShouldNot(ContainSubstring("Waiting"))
				Expect(phase).ShouldNot(ContainSubstring("Running"))
			}

		},
		Entry("No phase set", testCaseGetRemediationPhase{
			Metal3Remediation: &infrav1.Metal3Remediation{},
			Succeed:           false,
		}),
		Entry("Phase is set to Running", testCaseGetRemediationPhase{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Status: infrav1.Metal3RemediationStatus{
					Phase: "Running",
				},
			},
			Succeed: true,
		}),
		Entry("Phase is set to Waiting", testCaseGetRemediationPhase{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Status: infrav1.Metal3RemediationStatus{
					Phase: "Waiting",
				},
			},
			Succeed: true,
		}),
		Entry("Phase is set to Deleting", testCaseGetRemediationPhase{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Status: infrav1.Metal3RemediationStatus{
					Phase: "Deleting machine",
				},
			},
			Succeed: true,
		}),
		Entry("Phase is set to something else", testCaseGetRemediationPhase{
			Metal3Remediation: &infrav1.Metal3Remediation{
				Status: infrav1.Metal3RemediationStatus{
					Phase: "test",
				},
			},
			Succeed: false,
		}),
	)

})
