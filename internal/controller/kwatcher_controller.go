package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	corev1beta1 "kwatch.cloudcorner.org/k-watcher/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// KwatcherReconciler reconciles a Kwatcher object
type KwatcherReconciler struct {
	client.Client         //Fournit un accès aux ressources Kubernetes.
	Scheme                *runtime.Scheme
	MaxConcurrentRollouts int `default:"3"`
	RolloutQueue          workqueue.RateLimitingInterface
	rolloutLock           sync.Mutex
	activeRollouts        int
}

// +kubebuilder:rbac:groups=core.kwatch.cloudcorner.org,resources=kwatchers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.kwatch.cloudcorner.org,resources=kwatchers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.kwatch.cloudcorner.org,resources=kwatchers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;update;patch;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// la méthode Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) est automatiquement
// déclenchée chaque fois qu’une opération est effectuée sur un objet surveillé (watched object).
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Kwatcher object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *KwatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	l := log.FromContext(ctx)
	fmt.Println("START RECONCILING...")

	//crée une instance d’un objet Kwatcher défini dans le package corev1beta1 et retourne un pointeur vers cette instance pour modifier ses valeurs.
	instance := &corev1beta1.Kwatcher{}
	secret := &corev1.Secret{}
	myConfig := &corev1.ConfigMap{}

	//on'a passé instance en paramètre de la fonction Get() pour récupérer l'état actuel de l'objet Kwatcher.
	err := r.Get(ctx, req.NamespacedName, instance)
	if (err) != nil {
		fmt.Println("UNABLE TO FETCH KWATCHER")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	prov := instance.Spec.Provider
	conf := instance.Spec.Config

	if validateErr := instance.Spec.Validate(); validateErr != nil {
		return ctrl.Result{}, err
	}
	apiSecret, apiKeyType, isSecretNotExist, ctxRes, err1 := getKubeSecret(ctx, conf, instance, r, secret, l)
	if isSecretNotExist {
		return ctxRes, err1
	}

	// Call the external webservice method
	// Call the utility function instead of implementing it here
	jsonResponse, err := r.callWebService(ctx, prov, conf, apiKeyType, apiSecret)
	if err != nil {
		l.Error(err, "Failed to call webservice")
		return ctrl.Result{}, err
	}

	//fmt.Println("Received response from webservice", "result", jsonResponse)

	if err := r.createOrUpdateConfigMap(ctx, instance, myConfig, jsonResponse, l); err != nil {
		l.Error(err, "unable to create or update ConfigMap")
		return ctrl.Result{}, err
	}

	//asynchrone: - La goroutine permet de les exécuter en arrière-plan sans bloquer le contrôleur- La goroutine permet de les exécuter en arrière-plan sans bloquer le contrôleur.
	go r.processRolloutQueue(ctx)

	return ctrl.Result{RequeueAfter: time.Duration(instance.Spec.Config.RefreshInterval) * time.Second}, nil
}

func (r *KwatcherReconciler) createOrUpdateConfigMap(ctx context.Context, instance *corev1beta1.Kwatcher, myConfig *corev1.ConfigMap,
	jsonResponse map[string]interface{}, l logr.Logger) error {

	// 1. Convertir les données JSON en premier
	jsonData, err := convertToJson(jsonResponse)
	if err != nil {
		return err
	}

	// 2. Vérifier si le ConfigMap existe déjà
	err = r.Get(ctx, types.NamespacedName{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	}, myConfig)

	if client.IgnoreNotFound(err) != nil {
		l.Error(err, "Failed to fetch ConfigMap")
		return err
	}

	// 3. Préparer le ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Data: map[string]string{
			"config": string(jsonData),
		},
	}

	// 4. Créer ou mettre à jour
	if err != nil { // Not found
		if err := r.Create(ctx, configMap); err != nil {
			l.Error(err, "Failed to create ConfigMap")
			return err
		}
	} else {
		res := compareAndUpdateConfig(jsonData, myConfig)
		if res.Err != nil {
			return res.Err
		} else {
			if res.needUpdate {
				myConfig.Data = configMap.Data
				if err := r.Update(ctx, myConfig); err != nil {
					l.Error(err, "Failed to update ConfigMap")
					return err
				}
				// 5. Gestion des Deployments (inchangée)
				deployments := &appsv1.DeploymentList{}
				if err := r.List(ctx, deployments, client.InNamespace(instance.Namespace)); err != nil {
					l.Error(err, "Failed to list Deployments")
					return err
				}
				for _, dep := range deployments.Items {
					if shouldHandleDeployment(&dep, myConfig.Name) {
						r.RolloutQueue.Add(types.NamespacedName{
							Namespace: dep.Namespace,
							Name:      dep.Name,
						})
					}
				}
			}
		}

	}

	return nil
}

func getKubeSecret(ctx context.Context, conf corev1beta1.KwatcherConfig, instance *corev1beta1.Kwatcher, r *KwatcherReconciler, secret *corev1.Secret, l logr.Logger) (string, string, bool, ctrl.Result, error) {
	var apiSecret string
	var apiKeyType string
	if len(strings.TrimSpace(conf.Secret)) > 0 {
		secretKey := types.NamespacedName{
			Name:      conf.Secret,
			Namespace: instance.Namespace,
		}

		if err := r.Get(ctx, secretKey, secret); err != nil {
			l.Error(err, "Unable to fetch secret")
			return "", "", true, ctrl.Result{}, client.IgnoreNotFound(err)
		}
		apiKeyType = string(secret.Data["key-type"])
		apiSecret = string(secret.Data["client-key"])

		if apiKeyType == "" || apiSecret == "" {
			l.Error(nil, "Secret data is empty or invalid")
			return "", "", true, ctrl.Result{}, fmt.Errorf("secret data is empty or invalid")
		}

	}
	return apiSecret, apiKeyType, false, ctrl.Result{}, nil
}

func shouldHandleDeployment(deployment *appsv1.Deployment, configMapName string) bool {
	annotations := deployment.Spec.Template.Annotations
	if annotations == nil {
		return false
	}

	if policy, ok := annotations["kwatcher.config/update-policy"]; !ok || policy != "explicit" {
		return false
	}

	watched := strings.Split(annotations["kwatcher.config/watched-configmaps"], ",")
	for _, cm := range watched {
		if strings.TrimSpace(cm) == configMapName {
			return true
		}
	}

	return false
}

func (r *KwatcherReconciler) processRolloutQueue(ctx context.Context) {
	log := ctrl.LoggerFrom(ctx)
	for {
		item, shutdown := r.RolloutQueue.Get()
		if shutdown {
			break
		}

		key := item.(types.NamespacedName)
		r.RolloutQueue.Done(item)

		r.rolloutLock.Lock()
		if r.activeRollouts >= r.MaxConcurrentRollouts {
			r.RolloutQueue.AddRateLimited(key)
			r.rolloutLock.Unlock()
			continue
		}
		r.activeRollouts++
		r.rolloutLock.Unlock()

		go func() {
			defer func() {
				r.rolloutLock.Lock()
				r.activeRollouts--
				r.rolloutLock.Unlock()
			}()

			if err := r.triggerRollout(ctx, key); err != nil {
				log.Error(err, "Rollout failed", "deployment name: ", key.Name)
				r.RolloutQueue.AddRateLimited(key)
			} else {
				r.RolloutQueue.Forget(key)
			}
		}()
	}
}

func (r *KwatcherReconciler) triggerRollout(ctx context.Context, key types.NamespacedName) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, key, deployment); err != nil {
		return fmt.Errorf("failed to fetch deployment: %w", err)
	}

	// // Vérifier la stratégie de déploiement
	// strategy := deployment.Spec.Template.Annotations["operator.config/strategy"]
	// if strategy == "canary" {
	// 	return r.canaryRollout(ctx, deployment)
	// }

	// Stratégie par défaut: rolling update
	return r.standardRollout(ctx, deployment)
}

func (r *KwatcherReconciler) standardRollout(ctx context.Context, deployment *appsv1.Deployment) error {
	patch := client.MergeFrom(deployment.DeepCopy())
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kwatcher.config/last-updated"] = time.Now().Format(time.RFC3339)
	return r.Patch(ctx, deployment, patch)
}

// SetupWithManager sets up the controller with the Manager.
func (r *KwatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1beta1.Kwatcher{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 2,                                                                                                // Limits the controller to running 3 reconciliation loops simultaneously**
			RateLimiter:             workqueue.NewTypedItemExponentialFailureRateLimiter[ctrl.Request](1*time.Second, 60*time.Second), //***
		}).
		Named("kwatcher").
		Complete(r)
}

//** If 10 ConfigMaps change simultaneously, so 2 will begin processing immediately and 8 will wait until the 2 that are already processing have finished.
//*** The RateLimiter is a mechanism that controls how often a resource is reconciled, and it's used to prevent the controller from overloading the Kubernetes API server.
