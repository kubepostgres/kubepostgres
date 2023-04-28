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

package db

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dbv1alpha1 "github.com/kubepostgres/kubepostgres/api/db/v1alpha1"
)

// DatabaseReconciler reconciles a Database object
type DatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db.kubepostgres.dev,resources=databases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db.kubepostgres.dev,resources=databases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db.kubepostgres.dev,resources=databases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile
func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	l.Info("starting Database reconciliation")

	var database dbv1alpha1.Database
	l.V(1).Info("getting database", "NamespacedName", req.NamespacedName, "Database spec", database.Spec)
	if err := r.Get(ctx, req.NamespacedName, &database); err != nil {
		l.Error(err, "unable to fetch Database")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ss := newStatefulSet(&database)
	l.V(1).Info("getting StatefulSet", "StatefulSet", ss, "StatefulSet spec", ss.Spec)
	// check if the StatefulSet already exists
	err := r.Client.Get(ctx, types.NamespacedName{Name: ss.Name, Namespace: ss.Namespace}, ss)
	if err != nil {
		if errors.IsNotFound(err) {
			l.V(1).Info("creating a new StatefulSet")
			err = r.Client.Create(ctx, ss)
			if err != nil {
				return reconcile.Result{}, err
			}
		} else {
			return reconcile.Result{}, err
		}
	}
	l.V(1).Info("setting controller reference", "StatefulSet", ss, "StatefulSet spec", ss.Spec, "scheme", r.Scheme)
	// set Database instance as the owner and controller
	if err := controllerutil.SetControllerReference(&database, ss, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}

	service := newService(&database)
	l.V(1).Info("getting Service", "Service", service, "Service spec", service.Spec)
	// Check if the Service already exists
	err = r.Client.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, service)
	if err != nil {
		if errors.IsNotFound(err) {
			l.V(1).Info("creating a new Service")
			err = r.Client.Create(ctx, service)
			if err != nil {
				return reconcile.Result{}, err
			}
		} else {
			return reconcile.Result{}, err
		}
	}
	l.V(1).Info("setting controller reference", "Service", service, "Service spec", service.Spec, "scheme", r.Scheme)
	// set Database instance as the owner and controller
	if err := controllerutil.SetControllerReference(&database, service, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbv1alpha1.Database{}).
		Complete(r)
}

func newStatefulSet(database *dbv1alpha1.Database) *appsv1.StatefulSet {
	labels := map[string]string{
		"app": database.Name,
	}
	containerPorts := []corev1.ContainerPort{{
		ContainerPort: 5432,
		Protocol:      corev1.ProtocolTCP,
	}}
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      database.Name + "-postgres",
			Namespace: database.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      database.Name + "-postgres",
					Namespace: database.Namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "postgres",
							Image: database.Spec.ContainerImage,
							Ports: containerPorts,
							Env: []corev1.EnvVar{
								{
									Name:  "POSTGRES_PASSWORD",
									Value: "password",
								},
							},
						},
					},
				},
			},
		},
	}
	return ss
}

func newService(database *dbv1alpha1.Database) *corev1.Service {
	labels := map[string]string{
		"app": database.Name,
	}
	var svcPorts []corev1.ServicePort
	svcPort := corev1.ServicePort{
		Name:       database.Name + "-postgres",
		Port:       5432,
		Protocol:   corev1.ProtocolTCP,
		TargetPort: intstr.FromInt(5432),
	}
	svcPorts = append(svcPorts, svcPort)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      database.Name + "-postgres",
			Namespace: database.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: svcPorts,
			Selector: map[string]string{
				"app": database.Name,
			},
		},
	}
	return svc
}
