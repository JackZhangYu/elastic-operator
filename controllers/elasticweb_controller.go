/*
Copyright 2022 chengyu.

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

	appsv1 "k8s.io/api/apps/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mallv1 "mall/api/v1"
)

// ElasticWebReconciler reconciles a ElasticWeb object
type ElasticWebReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mall.mall.com,resources=elasticwebs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mall.mall.com,resources=elasticwebs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mall.mall.com,resources=elasticwebs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ElasticWeb object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *ElasticWebReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &mallv1.ElasticWeb{}

	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info(fmt.Sprintf("instance:%s", instance.String()))

	// ??????deployment
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, deploy); err != nil {
		if errors.IsNotFound(err) {
			// ???????????????????????????????????????
			logger.Info("deploy not exists")
			// ??????qps??????????????????qps??????????????????????????????
			if *instance.Spec.TotalQPS < 1 {
				logger.Info("not need deployment")
				return ctrl.Result{}, nil
			}

			// ??????service
			if err = CreateServiceIfNotExists(ctx, r, instance, req); err != nil {
				return ctrl.Result{}, err
			}

			// ??????Deploy
			if err := CreateDeployment(ctx, r, instance); err != nil {
				return ctrl.Result{}, err
			}

			// ????????????
			if err := UpdateStatus(ctx, r, instance); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get deploy")
		return ctrl.Result{}, err
	}

	// ????????????Pod???QPS????????????pod?????????
	expectReplicas := getExpectReplicas(instance)

	// ????????????deployment?????????pod??????
	realReplicas := deploy.Spec.Replicas

	if expectReplicas == *realReplicas {
		logger.Info("not need to reconcile")
		return ctrl.Result{}, nil
	}

	// ????????????
	deploy.Spec.Replicas = &expectReplicas
	// ?????? deploy
	if err := r.Update(ctx, deploy); err != nil {
		logger.Error(err, "update deploy replicas error")
		return ctrl.Result{}, err
	}

	// ????????????
	if err := UpdateStatus(ctx, r, instance); err != nil {
		logger.Error(err, "update status error")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElasticWebReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		For(&mallv1.ElasticWeb{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
