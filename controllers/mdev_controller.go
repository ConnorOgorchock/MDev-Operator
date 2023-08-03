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
	mdevv1alpha1 "github.com/api/v1alpha1"
	"github.com/google/uuid"
	"io/ioutil"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

// MDevReconciler reconciles a MDev object
type MDevReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	//	Log      logr.Logger
}

//+kubebuilder:rbac:groups=mdev.my.domain,resources=mdevs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mdev.my.domain,resources=mdevs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mdev.my.domain,resources=mdevs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MDev object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *MDevReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	//	log := r.Log.WithValues("mdev", req.NamespacedName)

	mdev := &mdevv1alpha1.MDev{}
	err := r.Get(ctx, req.NamespacedName, mdev)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Mdev resource not found. Ignoring since object must be deleted")

			deleteAllMDEVS()

			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get MDev")
		return ctrl.Result{}, err
	}

	/*			found := appsv1.Deployment{}
				err = r.Get(ctx, req.NamespacedName, found)
				if err != nil && errors.IsNotFound(err) {
					// Define a new deployment
					dep := r.deploymentForMDev(mdev)
					log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
					err = r.Create(ctx, dep)
					if err != nil {
						log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
						return ctrl.Result{}, err
					}
					// Deployment created successfully - return and requeue
					return ctrl.Result{Requeue: true}, nil
				} else if err != nil {
					log.Error(err, "Failed to get Deployment")
					return ctrl.Result{}, err
				}
	*/

	//get new device type from CR
	//this is a list to support multiple types in the CR (later)
	desiredmdevs := mdev.Spec.MDevTypes

	// list of all gpus to iterate over
	gpus, _ := ioutil.ReadDir("/sys/class/mdev_bus/")
	for _, gpu := range gpus {

		for j, desiredmdevtype := range desiredmdevs {
			devicepath := filepath.Join("/sys/class/mdev_bus/", gpu.Name(), "/mdev_supported_types", desiredmdevtype, "")
			if _, err := exists(devicepath); err == nil {
				//create
				createMDEVS(desiredmdevtype, devicepath)
				//change slice for next loop
				desiredmdevs = append(desiredmdevs[j+1:], desiredmdevs[:j+1]...)
				//break from inner loop
				break
			}
		}
	}

	//	devicepath := filepath.Join("/sys/class/mdev_bus/0000:65:00.0/mdev_supported_types", desiredmdevs[0], "")

	//	instpath := filepath.Join(devicepath, "available_instances")
	//	instances, err := os.ReadFile(instpath)
	//	if err != nil {
	//		log.Error(err, "failed to get available instances\n")
	//	}

	//	for i, _ := strconv.Atoi(string(instances)); i > 0; i-- {

	//		log.Info("LOOP: %d", i)

	//add device to list of existing mdevs
	//	mdev.Status.MDevs = append(mdev.Status.MDevs, &newMDEV)

	//	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MDevReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mdevv1alpha1.MDev{}).
		Complete(r)
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func createMDEVS(mdevtype, mdevpath string) {

	file, _ := os.OpenFile(filepath.Join(mdevpath, "available_instances"), os.O_RDONLY, 0200)
	var instances int
	fmt.Fscanf(file, "%d", &instances)
	file.Close()

	for i := 0; i < instances; i++ {

		//initialize struct
		newMDEV := mdevv1alpha1.MDEV{
			UUID:     uuid.New().String(),
			TypeName: mdevtype,
			FilePath: mdevpath,
		}

		//try to create device
		path := filepath.Join(newMDEV.FilePath, "create")
		f, err := os.OpenFile(path, os.O_WRONLY, 0200)
		if err != nil {
			//		log.Error(err, "failed to create mdev type, can't open path\n")
		}

		if _, err = f.WriteString(newMDEV.UUID); err != nil {
			//		log.Error(err, "failed to create mdev type, can't write to path\n")
		}
		f.Close()
		time.Sleep(1 * time.Second)
	}
}

//brute force delete all mdevs
func deleteAllMDEVS() {

	gpus, _ := ioutil.ReadDir("/sys/class/mdev_bus/")
	for _, gpu := range gpus {
		supportedpath := filepath.Join("/sys/class/mdev_bus/", gpu.Name(), "/mdev_supported_types/")
		supportedTypes, _ := ioutil.ReadDir(supportedpath)
		for _, stype := range supportedTypes {
			devicespath := filepath.Join(supportedpath, stype.Name(), "/devices/")
			devices, _ := ioutil.ReadDir(devicespath)
			for _, device := range devices {
				removepath := filepath.Join(devicespath, device.Name(), "/remove")
				f, _ := os.OpenFile(removepath, os.O_WRONLY, 0200)
				f.WriteString("1")
				time.Sleep(1 * time.Second)
				f.Close()
			}
		}
	}
}
