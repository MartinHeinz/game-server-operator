## Initial Setup

```shell
operator-sdk init --domain martinheinz.dev --repo=github.com/MartinHeinz/game-server-operator --owner="Martin Heinz" --license=none
operator-sdk create api --group gameserver --version v1alpha1 --kind Server
```

# Development Environment

```shell
kind delete cluster --name kind
kind create cluster --config config/kind/kind-config.yaml --image=kindest/node:v1.20.0
kind export kubeconfig

make deploy IMG=$IMAGE
kubectl get crd
NAME                                  CREATED AT
servers.gameserver.martinheinz.dev    2021-01-06T13:03:10Z

kubectl get pods -n game-server-operator-system
NAME                                                       READY   STATUS             RESTARTS   AGE
game-server-operator-controller-manager-6c7758447b-qnlhq   2/2     Running            0          8s
```

## Deployment

```shell
export USERNAME=martinheinz
export IMAGE=docker.io/$USERNAME/game-server-operator:v0.0.1

docker build -t $IMAGE .
docker push $IMAGE # kind load docker-image $IMAGE
make deploy IMG=$IMAGE

kubectl create -f config/samples/gameserver_v1alpha1_server.yaml
kubectl get pods
```