/*
Copyright 2023.

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

package controllers

import (
	"context"
	"fmt"
	"reflect"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	secretv1alpha1 "github.com/nakamasato/secret-mirror-operator/api/v1alpha1"
)

// SecretMirrorReconciler reconciles a SecretMirror object
type SecretMirrorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=secret.nakamasato.com,resources=secretmirrors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=secret.nakamasato.com,resources=secretmirrors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=secret.nakamasato.com,resources=secretmirrors/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=watch;list;get;create;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SecretMirror object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *SecretMirrorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// 1. Get SecretMirror from request.
	secretMirror := &secretv1alpha1.SecretMirror{}
	err := r.Get(ctx, req.NamespacedName, secretMirror)

	// 2. If SecretMirror doesn't exist, just finish the reconciliation. If error occurs, retry later.
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("[SecretMirror] Not found")
			return ctrl.Result{}, nil
		}
		log.Error(err, "[SecretMirror] Failed to fetch")
		return ctrl.Result{}, err
	}

	// 3. Get Secret (`fromSecret`) with SecretMirror's name from `fromNamespace` Namespace.
	fromSecret := &v1.Secret{}
	err = r.Get(ctx, client.ObjectKey{Namespace: secretMirror.Spec.FromNamespace, Name: secretMirror.Name}, fromSecret)

	// 4. If Secret doesn't exist, delete the corresponding Secret (`toSecret`) if exists. If error occurs, retry later.
	if err != nil {
		if errors.IsNotFound(err) {
			toSecret := &v1.Secret{}
			err = r.Get(ctx, client.ObjectKey{Namespace: secretMirror.Namespace, Name: secretMirror.Name}, toSecret)
			if err != nil {
				log.Error(err, "[toSecret] Failed to get")
				return ctrl.Result{}, err
			}
			if !metav1.IsControlledBy(toSecret, secretMirror) {
				log.Error(err, "[toSecret] Not controlled by SecretMirror")
				return ctrl.Result{}, nil
			}
			err := r.Delete(ctx, toSecret, &client.DeleteOptions{})
			if err != nil {
				log.Error(err, "[toSecret] Failed to delete")
				return ctrl.Result{}, err
			}
			log.Info(fmt.Sprintf("[fromSecret] Not found in %s and deleted toSecret in %s", secretMirror.Spec.FromNamespace, secretMirror.Namespace))
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, fmt.Sprintf("[fromSecret] Failed to fetch from %s", secretMirror.Spec.FromNamespace))
		return ctrl.Result{}, err
	}

	// 5. Create `toSecret` if not exists.
	toSecret := &v1.Secret{}
	err = r.Get(ctx, client.ObjectKey{Namespace: secretMirror.Namespace, Name: secretMirror.Name}, toSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			toSecret = newSecret(secretMirror, fromSecret)
			err := ctrl.SetControllerReference(secretMirror, toSecret, r.Scheme)
			if err != nil {
				log.Error(err, "[toSecret] Failed to set controller reference")
				return ctrl.Result{}, err
			}
			err = r.Create(ctx, toSecret, &client.CreateOptions{})
			if err != nil {
				log.Error(err, "[toSecret] Failed to create")
				return ctrl.Result{}, err
			}
			log.Info("[toSecret] Created")
			return ctrl.Result{}, nil
		}
		log.Error(err, "[toSecret] Failed to get")
		return ctrl.Result{}, err
	}

	// 6. Check if `toSecret` is managed by secret-mirror-controller.
	if !metav1.IsControlledBy(toSecret, secretMirror) {
		log.Error(err, "[toSecret] Not controlled by SecretMirror")
		return ctrl.Result{}, err
	}

	// 7. Update toSecret data if data is changed.
	if !reflect.DeepEqual(toSecret.Data, fromSecret.Data) {
		toSecret.Data = fromSecret.Data
		err = r.Update(ctx, toSecret, &client.UpdateOptions{})
		if err != nil {
			log.Error(err, "[toSecret] Failed to update")
		}
		log.Info("[toSecret] Updated with fromSecret.Data")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretMirrorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&secretv1alpha1.SecretMirror{}).
		Owns(&v1.Secret{}).
		Watches(
			&source.Kind{Type: &v1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
				secretMirrorList := &secretv1alpha1.SecretMirrorList{}
				ctx := context.Background()
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()
				mgr.GetCache().List(ctx, secretMirrorList)
				requests := []reconcile.Request{}
				for _, item := range secretMirrorList.Items {
					if item.Spec.FromNamespace == a.GetNamespace() && item.Name == a.GetName() {
						requests = append(requests,
							reconcile.Request{
								NamespacedName: types.NamespacedName{
									Name:      item.Name,
									Namespace: item.Namespace,
								},
							})
					}
				}
				return requests
			}),
		).
		Complete(r)
}

func newSecret(secretMirror *secretv1alpha1.SecretMirror, fromSecret *v1.Secret) *v1.Secret {
	toSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretMirror.Name,
			Namespace: secretMirror.Namespace,
		},
		Data: fromSecret.Data,
	}
	return toSecret
}
