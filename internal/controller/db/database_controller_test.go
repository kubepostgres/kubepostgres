/*
Copyright 2023 The KubePostgres Authors.

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

package db_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	dbv1alpha1 "github.com/kubepostgres/kubepostgres/api/db/v1alpha1"
)

var _ = Describe("Creating Database:", func() {

	const (
		timeout       = time.Second * 20
		interval      = time.Millisecond * 250
		testNamespace = "default"
	)

	Context("When creating Database with default values,", func() {

		AfterEach(func() {
		})

		It("it should create a StatefulSet and Service", func() {
			ctx := context.Background()
			By("creating Database")
			database := &dbv1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db1",
					Namespace: testNamespace,
				},
				Spec: dbv1alpha1.DatabaseSpec{
					ContainerImage: "postgres",
				},
			}
			Expect(k8sClient.Create(ctx, database)).Should(Succeed())

			ssLookupKey := types.NamespacedName{Name: "db1-postgres", Namespace: testNamespace}
			ss := &appsv1.StatefulSet{}

			Eventually(func() bool {
				return k8sClient.Get(ctx, ssLookupKey, ss) == nil
			}, timeout, interval).Should(BeTrue())

			serviceLookupKey := types.NamespacedName{Name: "db1-postgres", Namespace: testNamespace}
			service := &corev1.Service{}

			Eventually(func() bool {
				return k8sClient.Get(ctx, serviceLookupKey, service) == nil
			}, timeout, interval).Should(BeTrue())

		})
	})
})
