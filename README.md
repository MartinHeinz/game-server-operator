# Game Server Operator

This is repository for Kubernetes game-server-operator. This operator allows you to deploy popular game servers with single YAML (CRD).

Currently supported game servers are: CS:GO, Rust, Minecraft and Factorio. Any containerized game server can be easily added.

The most minimalistic server configuration can be as simple as:

```yaml
apiVersion: gameserver.martinheinz.dev/v1alpha1
kind: Server
metadata:
  name: csgo
spec:
  serverName: csgo
  gameName: "CSGO"
  envFrom:
    - configMapRef:
        name: csgo
    - secretRef:
        name: csgo
  storage:
    size: 12Gi
```

For sample configurations for each game see [samples directory](./config/samples)

For details on how to setup and connect to each game server see [Games section below](#games)

## Initial Setup

```shell
operator-sdk init --domain martinheinz.dev --repo=github.com/MartinHeinz/game-server-operator --owner="Martin Heinz" --license=none
operator-sdk create api --group gameserver --version v1alpha1 --kind Server
operator-sdk create webhook --group gameserver --version v1alpha1 --kind Server --defaulting --programmatic-validation
```

# Deployment (on KinD)

```shell
kind delete cluster --name operator
kind create cluster --name operator --config config/kind/kind-config.yaml --image=kindest/node:v1.20.0
kind --name operator export kubeconfig

# Install cert-manager
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.1.0/cert-manager.yaml
kubectl get pods --namespace cert-manager
NAME                                      READY   STATUS    RESTARTS   AGE
cert-manager-5597cff495-d8mmx             1/1     Running   0          34s
cert-manager-cainjector-bd5f9c764-mssm2   1/1     Running   0          34s
cert-manager-webhook-5f57f59fbc-m8j2j     1/1     Running   0          34s

export USERNAME=martinheinz
export IMAGE=docker.io/$USERNAME/game-server-operator:latest

docker build -t $IMAGE .  # ONLY FOR DEVELOPEMENT
docker push $IMAGE        # ONLY FOR DEVELOPEMENT

make deploy IMG=$IMAGE
kubectl get crd
NAME                                  CREATED AT
servers.gameserver.martinheinz.dev    2021-01-06T13:03:10Z

kubectl get pods -n game-server-operator-system
NAME                                                       READY   STATUS             RESTARTS   AGE
game-server-operator-controller-manager-6c7758447b-qnlhq   2/2     Running            0          8s

# Options: rust, csgo, minecraft, factorio
export GAME_NAME=...
# For game-specific notes, see _Games_ section below
kubectl apply -f config/samples/${GAME_NAME}.yaml

kubectl get server ${GAME_NAME}
NAME        STATUS   STORAGE   AGE
GAME_NAME   Active   Bound     39h
```

# Games

Before we can deploy individual servers, we first need to deploy the operator, for that see [Deployment section](#deployment-on-kind) 

## CS:GO

- Create server (modify file to override defaults):
```shell
~ $ kubectl apply -f config/sample/rust.yaml
```

- Verify that the server is running:
```shell
~ $ kubectl get server csgo
NAME   STATUS   STORAGE   AGE
csgo   Active   Bound     39h
```

- Connecting to server (if running on _KinD_):
```shell
~ $ docker inspect --format='{{.NetworkSettings.IPAddress}}' operator-control-plane
172.17.0.2  # Node IP
```

By default CS:GO uses port `27015`, which is exposed using NodePort at `30015`.

- Start game and open console (using _tilde_ key)
- Type:
```
rcon_address 172.17.0.2:30015
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

Playing on server:

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

## Rust

Create server (modify file to override defaults):
```shell
~ $ kubectl apply -f config/sample/rust.yaml
```

Verify that the server is running:
```shell
~ $ kubectl get server rust
NAME   STATUS   STORAGE   AGE
rust   Active   Bound     39h

~ $ kubectl logs deploy/rust-deployment
...
WebSocket RCon Started on 30016
...

~ $ kubectl exec deploy/rust-deployment -- rcon status
RconApp::Relaying RCON command: status
RconApp::Received message: hostname: My Awesome Server
version : 2275 secure (secure mode enabled, connected to Steam3)
map     : Procedural Map
players : 0 (500 max) (0 queued) (0 joining)

id name ping connected addr owner violation kicks 

RconApp::Command relayed
```

Connecting to server (if running on _KinD_):
```shell
~ $ docker inspect --format='{{.NetworkSettings.IPAddress}}' operator-control-plane
172.17.0.2  # Node IP
```

By default ports are set to: 
- `30015` - user access
- `30016` - RCON access
- `30080` - RCON browser access

## Factorio

Create server (modify file to override defaults):
```shell
~ $ kubectl apply -f config/sample/factorio.yaml
```

Verify that the server is running:
```shell
~ $ kubectl get server factorio
NAME       STATUS   STORAGE   AGE
factorio   Active   Bound     80s
```

Testing RCON connection with Python:

```python
#  pip install factorio-rcon-py
import factorio_rcon

client = factorio_rcon.RCONClient("172.17.0.2", 30015, "<PASSWORD>")
response = client.send_command("/help")
```

The above snippet should exit with `code 0` and in logs you should see something like:

```
543.988 Info RemoteCommandProcessor.cpp:242: New RCON connection from IP ADDR:({...:54278})
```

By default ports are set to: 
- `32197` - user access
- `30015` - RCON access

## Minecraft

Create server (modify file to override defaults):
```shell
~ $ kubectl apply -f config/sample/minecraft.yaml
```

Verify that the server is running:
```shell
~ $ kubectl get server minecraft 
NAME        STATUS   STORAGE   AGE
minecraft   Active   Bound     10m
```

Test RCON:
```shell
~ $ kubectl exec --stdin --tty deploy/minecraft-deployment -- /bin/bash
bash-4.4# rcon-cli
> /list
There are 0 of a max of 20 players online:
...
CTRL+D
```

Logs should show something like:

```
[16:47:28] [RCON Listener #1/INFO]: Thread RCON Client /0:0:0:0:0:0:0:1 started
[16:48:06] [RCON Client /0:0:0:0:0:0:0:1 #3/INFO]: Thread RCON Client /0:0:0:0:0:0:0:1 shutting down
```

By default ports are set to: 
- `30565` - user access
- `30575` - RCON access

## Changing Default Server Configuration

| Parameter                 | Description                                                                                          |
|---------------------------|------------------------------------------------------------------------------------------------------|
| gameName                  | Name of the game. One of the `CSGO`, `Rust`, `Factorio`, `Minecraft`                                 |
| envFrom.configMapRef.name | Name of configMap used for configuration                                                             |
| envFrom.secretRef.name    | Name of Secret used for configuration of sensitive information                                       |
| port.targetPort           | Port that container is listening on (Optional)                                                       |
| port.port                 | Port exposed by generated Service (Optional)                                                         |
| port.nodePort             | Port that will be publicly exposed on cluster node (Optional)                                        |
| storage.size              | Size of PVC, e.g. `12Gi`                                                                             |
| resources.requests        | Minimum resources allocated for server container (Optional, Defaults to `memory: 64Mi`, `cpu: 128m`) |
| resources.limits          | Minimum resources available for server container (Optional, Defaults to `memory: 1`, `cpu: 1Gi`)     |


Complete configuration example:

```yaml
apiVersion: gameserver.martinheinz.dev/v1alpha1
kind: Server
metadata:
  name: csgo
spec:
  serverName: csgo
  gameName: "CSGO"
  port:  # These ports are defaults, can be omitted
    - port: 27015
      targetPort: 27015
      nodePort: 30015
  envFrom:
    - configMapRef:
        name: csgo
    - secretRef:
        name: csgo
  storage:
    size: 12Gi
  resources:
    requests:
      memory: "64Mi"
      cpu: "250m"
    limits:
      memory: "1Gi"
      cpu: "2"
```

To see what environment variables can be used in configMap and Secret for each game see:

- [Rust configuration options](https://github.com/Didstopia/rust-server#how-to-run-the-server)
- [CS:GO configuration options](https://github.com/kaimallea/csgo#environment-variable-overrides)
- [Minecraft configuration options](https://github.com/itzg/docker-minecraft-server/blob/master/README.md)

## Contributing

If you'd like to use this operator to deploy some other game server, then you can open an issue or contribute it yourself. That would include:

- Providing a Docker image for the game server
- Adding template (e.g. _Deployment_, _Service_ and _PVC_) for game in [server_controller.go](controllers/server_controller.go) (see `Games = map[gameserverv1alpha1.GameName]GameSetting{...}` near the end of file)
- Adding game name to list of possible server name values in [server_types.go](api/v1alpha1/server_types.go) (see `type GameName string`)