## Initial Setup

```shell
operator-sdk init --domain martinheinz.dev --repo=github.com/MartinHeinz/game-server-operator --owner="Martin Heinz" --license=none
operator-sdk create api --group gameserver --version v1alpha1 --kind Server
```

# Development Environment

```shell
kind delete cluster --name operator
kind create cluster --name operator --config config/kind/kind-config.yaml --image=kindest/node:v1.20.0
kind --name operator export kubeconfig
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/master/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s

make deploy IMG=$IMAGE
kubectl get crd
NAME                                  CREATED AT
servers.gameserver.martinheinz.dev    2021-01-06T13:03:10Z

kubectl apply -f config/samples/config.yaml
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

kubectl apply -f config/samples/config.yaml
kubectl create -f config/samples/gameserver_v1alpha1_server.yaml
kubectl get pods
```

## Testing and Connecting

With `kubectl port-forward`:

```shell
~ $ kubectl port-forward <pod-name> 27015:27015
Forwarding from 127.0.0.1:27015 -> 27015
Forwarding from [::1]:27015 -> 27015
```

In CS:GO:

- Open console (using _tilde_ key)
- Type:
```
rcon_address 127.0.0.1:27015
rcon_password <RCON_PASSWORD field in csgo Secret>
rcon status
```

This should output something like:

```
hostname: csgo.default.svc.cluster.local
version : 1.37.7.5/13775 1215/8012 secure  [G:1:3964911] 
udp/ip  : 0.0.0.0:27015  (public ip: ...)
os      :  Linux
type    :  community dedicated
map     : de_dust2
gotv[0]:  port 27020, delay 30.0s, rate 64.0
players : 0 humans, 1 bots (12/0 max) (hibernating)

# userid name uniqueid connected ping loss state rate adr
# 2 "GOTV" BOT active 64
#end
```

Using Kubernetes Service NodePort:

By default CS:GO uses port `27015`, which is exposed using NodePort at `30015`.

To connect to server running in KinD:

```shell
# Node IP
docker inspect --format='{{.NetworkSettings.IPAddress}}' operator-control-plane
172.17.0.2
```

- Open console in CS:GO (using _tilde_ key)
- Type (assuming IP above and default config):
```
password <SERVER_PASSWORD in Secret>  # Omit if using password-less server
connect 172.17.0.2:30015
```

Check server logs:

```shell
L 01/09/2021 - 09:02:59: "USERNAME<3><STEAM_1:1:11111111><>" connected, address ""
Client "USERNAME" connected (10.128.0.340:18320).
Server waking up from hibernation
```