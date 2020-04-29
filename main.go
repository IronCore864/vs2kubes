package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/kelseyhightower/envconfig"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Config is the struct for environment configuration
type Config struct {
	VaultAddr       string `envconfig:"VAULT_ADDR"`
	K8sNamespace    string `envconfig:"K8S_NAMESPACE"`
	VaultSecretPath string `envconfig:"VAULT_SECRET_PATH"`
	RoleID          string `envconfig:"VAULT_ROLE_ID"`
	SecretID        string `envconfig:"VAULT_SECRET_ID"`
	KvVersion       int    `envconfig:"VAULT_KV_VERSION"`
}

func getK8sClientSet() *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset
}

func buildK8sSecret(name string, data map[string][]byte) corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: data,
		Type: "Opaque",
	}
}

func upcertSecret(c *kubernetes.Clientset, name, namespace string, secret *corev1.Secret) error {
	_, err := c.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if strings.Index(err.Error(), "not found") >= 0 {
			_, err = c.CoreV1().Secrets(namespace).Create(secret)
			if err != nil {
				return err
			}
		}
	} else {
		_, err = c.CoreV1().Secrets(namespace).Update(secret)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	// load config
	var conf Config
	err := envconfig.Process("", &conf)
	if err != nil {
		log.Fatal(err.Error())
	}

	// kv v1 vs v2
	listSecretPath := ""
	readSecretPath := ""
	if conf.KvVersion == 1 {
		listSecretPath = fmt.Sprintf("%s/", conf.VaultSecretPath)
		readSecretPath = fmt.Sprintf("%s", conf.VaultSecretPath)
	} else {
		listSecretPath = fmt.Sprintf("%s/metadata/", conf.VaultSecretPath)
		readSecretPath = fmt.Sprintf("%s/data", conf.VaultSecretPath)
	}

	// vault client
	vaultClient, err := api.NewClient(&api.Config{
		Address: conf.VaultAddr,
	})
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	// AppRole auth
	c := vaultClient.Logical()
	data := map[string]interface{}{
		"role_id":   conf.RoleID,
		"secret_id": conf.SecretID,
	}
	resp, err := c.Write("auth/approle/login", data)
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	if resp.Auth == nil {
		log.Fatal("no auth info returned")
		return
	}
	// set token after AppRole auth
	vaultClient.SetToken(resp.Auth.ClientToken)

	// list vault secrets
	vaultSecrets, err := c.List(listSecretPath)
	if err != nil {
		log.Fatal(err.Error())
		return
	}

	// k8s client
	k8sClientset := getK8sClientSet()

	// iterate all vault secrets, generate k8s secret, and upcert
	switch x := vaultSecrets.Data["keys"].(type) {
	case []interface{}:
		for _, k := range x {
			secretName := fmt.Sprintf("%v", k)
			secret, err := c.Read(fmt.Sprintf("%s/%s", readSecretPath, secretName))
			if err != nil {
				log.Fatal(err.Error())
				return
			}

			data := make(map[string][]byte)
			for k, v := range secret.Data {
				value := fmt.Sprintf("%v", v)
				data[k] = []byte(value)
			}

			// upcert
			k8sSecret := buildK8sSecret(secretName, data)
			err = upcertSecret(k8sClientset, secretName, conf.K8sNamespace, &k8sSecret)
			if err != nil {
				log.Fatal(err.Error())
				panic(err.Error())
			}
		}
	}
}
