## Initial Setup

```shell
operator-sdk init --domain martinheinz.dev --repo=github.com/MartinHeinz/game-server-operator --owner="Martin Heinz" --license=none
operator-sdk create api --group gameserver --version v1alpha1 --kind Server
```