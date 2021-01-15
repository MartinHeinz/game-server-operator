package controllers

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	gameserverv1alpha1 "github.com/MartinHeinz/game-server-operator/api/v1alpha1"
)

// Run `go test ./...` in `controllers/` directory
var _ = Describe("Server controller", func() {

	// Define utility constants for object names and testing.
	const (
		ServerName      = "test-server"
		ServerNamespace = "default"

		DeploymentName = ServerName + "-deployment"
		ServiceName    = ServerName + "-service"
		PvcName        = ServerName + "-persistentvolumeclaim"
		ConfigMapName  = "csgo-env-config"
		SecretName     = "csgo-secret"
		GameName       = gameserverv1alpha1.CSGO

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		gameSettings GameSetting
	)

	BeforeEach(func() {

		for name, game := range Games {
			if name == GameName {
				gameSettings = game
				break
			}
		}

		ctx := context.Background()

		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ConfigMapName,
				Namespace: ServerNamespace,
			},
			Data: map[string]string{"SERVER_HOSTNAME": "hostname"},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      SecretName,
				Namespace: ServerNamespace,
			},
			StringData: map[string]string{"SERVER_PASSWORD": "password"},
		}

		Expect(k8sClient.Create(ctx, configMap)).Should(Succeed())
		Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

		configMapLookupKey := types.NamespacedName{Name: ConfigMapName, Namespace: ServerNamespace}
		createdConfigMap := &corev1.ConfigMap{}

		secretLookupKey := types.NamespacedName{Name: SecretName, Namespace: ServerNamespace}
		createdSecret := &corev1.Secret{}

		Eventually(func() bool {
			if err := k8sClient.Get(ctx, configMapLookupKey, createdConfigMap); err != nil {
				return false
			}
			if err := k8sClient.Get(ctx, secretLookupKey, createdSecret); err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue())

	})

	AfterEach(func() {
		ctx := context.Background()

		configMapLookupKey := types.NamespacedName{Name: ConfigMapName, Namespace: ServerNamespace}
		createdConfigMap := &corev1.ConfigMap{}

		secretLookupKey := types.NamespacedName{Name: SecretName, Namespace: ServerNamespace}
		createdSecret := &corev1.Secret{}

		Eventually(func() bool {
			if err := k8sClient.Get(ctx, configMapLookupKey, createdConfigMap); err != nil {
				return false
			}
			if err := k8sClient.Get(ctx, secretLookupKey, createdSecret); err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue())

		Expect(k8sClient.Delete(ctx, createdConfigMap)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, createdSecret)).Should(Succeed())
	})

	Context("When creating Server", func() {
		It("Should create objects with game-specific attributes", func() {
			By("creating a new Server")
			ctx := context.Background()

			resources := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("250m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}

			storage := &gameserverv1alpha1.ServerStorage{Size: "2G"}
			server := &gameserverv1alpha1.Server{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "servers.gameserver.martinheinz.dev/v1alpha1",
					Kind:       "Server",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      ServerName,
					Namespace: ServerNamespace,
				},
				Spec: gameserverv1alpha1.ServerSpec{
					ServerName: ServerName,
					GameName:   gameserverv1alpha1.CSGO,
					Ports: []corev1.ServicePort{
						{Name: "27015-tcp", Port: 27015, NodePort: 30020, TargetPort: intstr.IntOrString{Type: 0, IntVal: 27015, StrVal: ""}, Protocol: corev1.ProtocolTCP},
						{Name: "27015-udp", Port: 27015, NodePort: 30020, TargetPort: intstr.IntOrString{Type: 0, IntVal: 27015, StrVal: ""}, Protocol: corev1.ProtocolUDP},
					},
					EnvFrom: []corev1.EnvFromSource{{
						ConfigMapRef: &corev1.ConfigMapEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: ConfigMapName},
						},
					}, {
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: SecretName},
						},
					},
					},
					Storage:              storage,
					ResourceRequirements: resources,
				},
			}
			Expect(k8sClient.Create(ctx, server)).Should(Succeed())

			serverLookupKey := types.NamespacedName{Name: ServerName, Namespace: ServerNamespace}
			createdServer := &gameserverv1alpha1.Server{}

			deploymentLookupKey := types.NamespacedName{Name: DeploymentName, Namespace: ServerNamespace}
			createdDeployment := &appsv1.Deployment{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, serverLookupKey, createdServer); err != nil {
					return false
				}
				if err := k8sClient.Get(ctx, deploymentLookupKey, createdDeployment); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			createdContainer := createdDeployment.Spec.Template.Spec.Containers[0]
			Expect(createdContainer.Image).Should(Equal(gameSettings.Deployment.Spec.Template.Spec.Containers[0].Image))
			Expect(createdContainer.Ports).Should(Equal(gameSettings.Deployment.Spec.Template.Spec.Containers[0].Ports))
			Expect(createdContainer.EnvFrom).Should(Equal([]corev1.EnvFromSource{
				{ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: ConfigMapName,
					},
				}}, {SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: SecretName,
					},
				}},
			}))

			Expect(&createdContainer.Resources).Should(Equal(resources))

			serviceLookupKey := types.NamespacedName{Name: ServiceName, Namespace: ServerNamespace}
			createdService := &corev1.Service{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, serviceLookupKey, createdService); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(createdService.Spec.Selector["server"]).Should(Equal(ServerName))
			Expect(createdService.Spec.Ports).Should(Equal(server.Spec.Ports))

			pvcLookupKey := types.NamespacedName{Name: PvcName, Namespace: ServerNamespace}
			createdPvc := &corev1.PersistentVolumeClaim{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, pvcLookupKey, createdPvc); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(createdPvc.Spec.Resources.Requests.Storage().String()).Should(Equal(storage.Size))
			Expect(createdPvc.Name).Should(Equal(PvcName))

			// Check whether ClaimName was correctly assigned
			Expect(createdDeployment.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName).Should(Equal(PvcName))
		})
	})

	Context("When updating existing Server config", func() {
		It("Should modify deployment with new attributes", func() {
			By("updating a Server")
			ctx := context.Background()

			newConfigMapName := "csgo-env-config-new"

			newConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      newConfigMapName,
					Namespace: ServerNamespace,
				},
				Data: map[string]string{"SERVER_HOSTNAME": "new-hostname"},
			}
			Expect(k8sClient.Create(ctx, newConfigMap)).Should(Succeed())

			newConfigMapLookupKey := types.NamespacedName{Name: newConfigMapName, Namespace: ServerNamespace}
			createdNewConfigMap := &corev1.ConfigMap{}

			Eventually(func() bool {

				if err := k8sClient.Get(ctx, newConfigMapLookupKey, createdNewConfigMap); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			serverLookupKey := types.NamespacedName{Name: ServerName, Namespace: ServerNamespace}
			createdServer := &gameserverv1alpha1.Server{}

			deploymentLookupKey := types.NamespacedName{Name: DeploymentName, Namespace: ServerNamespace}
			createdDeployment := &appsv1.Deployment{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, serverLookupKey, createdServer); err != nil {
					return false
				}
				if err := k8sClient.Get(ctx, deploymentLookupKey, createdDeployment); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			deploymentGeneration := createdDeployment.Generation
			createdServer.Spec.EnvFrom = []corev1.EnvFromSource{
				{ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: newConfigMapName,
					},
				}}}

			Expect(k8sClient.Update(ctx, createdServer)).Should(Succeed())

			updatedServer := &gameserverv1alpha1.Server{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, serverLookupKey, updatedServer); err != nil {
					return false
				}
				if updatedServer.Generation < 2 {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			// New generation created
			Expect(updatedServer.Generation).Should(Equal(int64(2)))
			Expect(createdServer.Spec.EnvFrom).Should(Equal(updatedServer.Spec.EnvFrom))

			updatedDeployment := &appsv1.Deployment{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, deploymentLookupKey, updatedDeployment); err != nil {
					return false
				}
				if updatedDeployment.Generation <= deploymentGeneration {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			newCreatedContainer := updatedDeployment.Spec.Template.Spec.Containers[0]
			Expect(newCreatedContainer.EnvFrom).Should(Equal([]corev1.EnvFromSource{
				{ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: newConfigMapName,
					},
				}},
			}))
		})
	})

	Context("When updating existing Server ResourceRequirements", func() {
		It("Should modify deployment with new attributes", func() {
			By("updating a Server")
			ctx := context.Background()

			newResources := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			}

			serverLookupKey := types.NamespacedName{Name: ServerName, Namespace: ServerNamespace}
			createdServer := &gameserverv1alpha1.Server{}

			deploymentLookupKey := types.NamespacedName{Name: DeploymentName, Namespace: ServerNamespace}
			createdDeployment := &appsv1.Deployment{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, serverLookupKey, createdServer); err != nil {
					return false
				}
				if err := k8sClient.Get(ctx, deploymentLookupKey, createdDeployment); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			serverGeneration := createdServer.Generation
			deploymentGeneration := createdDeployment.Generation
			createdServer.Spec.ResourceRequirements = newResources

			Expect(k8sClient.Update(ctx, createdServer)).Should(Succeed())

			updatedServer := &gameserverv1alpha1.Server{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, serverLookupKey, updatedServer); err != nil {
					return false
				}
				if updatedServer.Generation <= serverGeneration {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			// New generation created
			Expect(updatedServer.Generation).Should(Equal(int64(3)))
			Expect(createdServer.Spec.EnvFrom).Should(Equal(updatedServer.Spec.EnvFrom))

			updatedDeployment := &appsv1.Deployment{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, deploymentLookupKey, updatedDeployment); err != nil {
					return false
				}
				if updatedDeployment.Generation <= deploymentGeneration {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			newCreatedContainer := updatedDeployment.Spec.Template.Spec.Containers[0]
			Expect(&newCreatedContainer.Resources).Should(Equal(newResources))
		})
	})

	Context("When updating existing Server NodePort", func() {
		It("Should modify service with new attribute", func() {
			By("updating a Server")
			ctx := context.Background()

			newNodePort := int32(30030)

			serverLookupKey := types.NamespacedName{Name: ServerName, Namespace: ServerNamespace}
			createdServer := &gameserverv1alpha1.Server{}

			serviceLookupKey := types.NamespacedName{Name: ServiceName, Namespace: ServerNamespace}
			createdService := &corev1.Service{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, serverLookupKey, createdServer); err != nil {
					return false
				}
				if err := k8sClient.Get(ctx, serviceLookupKey, createdService); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			createdServerGeneration := createdServer.Generation

			// Change and update NodePort of Server
			createdServer.Spec.Ports[0].NodePort = newNodePort
			Expect(k8sClient.Update(ctx, createdServer)).Should(Succeed())

			// Wait for Server to be updated (Generation increased)
			updatedServer := &gameserverv1alpha1.Server{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, serverLookupKey, updatedServer); err != nil {
					return false
				}
				if updatedServer.Generation <= createdServerGeneration {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			// New generation created
			Expect(updatedServer.Generation).Should(Equal(int64(4)))
			// New Generation of Server should have updated NodePort
			Expect(createdServer.Spec.Ports[0].NodePort).Should(Equal(updatedServer.Spec.Ports[0].NodePort))

			// Wait for child Service to be updated (Generation increased)
			updatedService := &corev1.Service{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, serviceLookupKey, updatedService); err != nil {
					return false
				}
				if updatedService.Spec.Ports[0].NodePort != newNodePort {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			// New Generation of Service should have updated NodePort
			updatedNodePort := updatedService.Spec.Ports[0].NodePort
			Expect(updatedNodePort).Should(Equal(newNodePort))
		})
	})
})
