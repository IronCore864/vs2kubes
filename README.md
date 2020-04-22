# vs2kubes

A golang app using vault API to get all secrets under a given path, then use k8s API create the secret.

At the moment, k8s client version is 1.15.11.

## Build

```
go build
```

## Usage

```
export VAULT_ADDR=https://vault.domain.com
export VAULT_TOKEN=YOUR_TOKEN
export K8S_NAMESPACE=default
# no need to put as "kv/", only "kv" is enough
export VAULT_SECRET_PATH=kv
# optional
export VAULT_SKIP_VERIFY=true
./vs2yaml
```

## Docker

```
ironcore864/vs2yaml:1.15.11
```
