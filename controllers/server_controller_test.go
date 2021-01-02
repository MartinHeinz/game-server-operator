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
		It("Should create Deployment and Service with game-specific image and ports", func() {
			By("By creating a new Server")
			ctx := context.Background()

			deploymentName := "csgo-server"
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
			Expect(createdDeployment.Spec.Template.Spec.Containers[0].Image).Should(Equal(csgo.container.Image))
			Expect(createdDeployment.Spec.Template.Spec.Containers[0].Ports).Should(Equal(csgo.container.Ports))

			serviceLookupKey := types.NamespacedName{Name: ServerName, Namespace: ServerNamespace}
			createdService := &corev1.Service{}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, serviceLookupKey, createdService); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(createdService.Spec.Selector["server"]).Should(Equal(ServerName))
			Expect(createdService.Spec.Ports).Should(Equal(csgo.servicePorts))

		})
	})
})
