# vs2kubes

Name: VaultSecretToKUBErnetesSecrets.

A golang app using vault API to get all secrets under a given path, then use k8s API create the secret.

At the moment, k8s client version is 1.15.11.

Only KV v2 engine is supported due to V1/V2 API differences.

Vault API uses AppRole auth. For configure, see: https://ironcore864.gitlab.io/#AppRole-Auth

Note that you must grant enough `token_num_uses` and `secret_id_num_uses`.

## Build

```
go build
```

## Usage

```
export VAULT_ADDR=https://vault.domain.com
export K8S_NAMESPACE=default
# no need to put as "kv/", only "kv" is enough
export VAULT_SECRET_PATH=kv
# optional
export VAULT_SKIP_VERIFY=true
# AppRole auth
export VAULT_ROLE_ID=ROLE_ID
export VAULT_SECRET_ID=SECRET_ID
./vs2yaml
```

## Docker

```
ironcore864/vs2yaml:1.15.11
```
