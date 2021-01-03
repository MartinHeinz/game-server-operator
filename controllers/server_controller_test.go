package controllers

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When updating Server Status", func() {
		It("Should create objects with game-specific attributes", func() {
			By("By creating a new Server")
			ctx := context.Background()

			deploymentName := "csgo-server"
			configMapName := "csgo-env-config"
			secretName := "csgo-secret"
			gameName := gameserverv1alpha1.CSGO

			var gameSettings GameSetting

			for i, game := range Games {
				if game.Name == gameName {
					gameSettings = Games[i]
					break
				}
			}

			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: ServerNamespace,
				},
				Data: map[string]string{"SERVER_HOSTNAME": "hostname"},
			}

			Expect(k8sClient.Create(ctx, configMap)).Should(Succeed())

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: ServerNamespace,
				},
				StringData: map[string]string{"SERVER_PASSWORD": "password"},
			}

			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			configMapLookupKey := types.NamespacedName{Name: configMapName, Namespace: ServerNamespace}
			createdConfigMap := &corev1.ConfigMap{}

			secretLookupKey := types.NamespacedName{Name: secretName, Namespace: ServerNamespace}
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

			storageSize := &gameserverv1alpha1.ServerStorage{Size: "2G"}
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
					ServerName: deploymentName,
					GameName:   gameserverv1alpha1.CSGO,
					Route:      deploymentName,
					Storage:    storageSize,
					EnvFrom: []corev1.EnvFromSource{{
						ConfigMapRef: &corev1.ConfigMapEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
						},
					}, {
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						},
					},
					},
				},
			}
			Expect(k8sClient.Create(ctx, server)).Should(Succeed())

			serverLookupKey := types.NamespacedName{Name: ServerName, Namespace: ServerNamespace}
			createdServer := &gameserverv1alpha1.Server{}

			deploymentLookupKey := types.NamespacedName{Name: ServerName, Namespace: ServerNamespace}
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
						Name: configMapName,
					},
				}}, {SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
				}},
			}))

			serviceLookupKey := types.NamespacedName{Name: ServerName, Namespace: ServerNamespace}
			createdService := &corev1.Service{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, serviceLookupKey, createdService); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(createdService.Spec.Selector["server"]).Should(Equal(ServerName))
			Expect(createdService.Spec.Ports).Should(Equal(gameSettings.Service.Spec.Ports))

			pvcLookupKey := types.NamespacedName{Name: ServerName, Namespace: ServerNamespace}
			createdPvc := &corev1.PersistentVolumeClaim{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, pvcLookupKey, createdPvc); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(createdPvc.Spec.VolumeName).Should(Equal(ServerName))
			Expect(createdPvc.Spec.Resources.Requests.Storage().String()).Should(Equal(storageSize.Size))

			// Check whether ClaimName was correctly assigned
			Expect(createdDeployment.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName).Should(Equal(ServerName))
		})
	})
})
